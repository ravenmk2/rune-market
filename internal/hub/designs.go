package hub

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/semver"

	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/store"
)

// DesignItem is the list-card read model (contract item shape).
type DesignItem struct {
	Design               *store.Designmd
	OwnerUsername        string
	OwnerNickname        string
	Tags                 []string
	LatestVersion        string
	PreviewDesktopSHA256 *string // of the latest version; handler maps to URLs
}

// DesignDetail is the detail read model.
type DesignDetail struct {
	DesignItem
	Latest *store.DesignmdVersion
}

type Designs struct {
	db      *sql.DB
	dialect string
	blobs   *blob.Storage
}

func NewDesigns(db *sql.DB, dialect string, blobs *blob.Storage) *Designs {
	return &Designs{db: db, dialect: dialect, blobs: blobs}
}

// DB exposes the connection pool for handlers that need direct access.
func (s *Designs) DB() *sql.DB { return s.db }

func (s *Designs) Tags(ctx context.Context) ([]string, error) {
	return store.NewTagStore(s.db, s.dialect).PublishedDesignTags(ctx)
}

// DesignPublishInput is one publish request (§8.4).
type DesignPublishInput struct {
	Owner          *store.User
	Name           string // artifact name, from the form
	Summary        string // form-filled (§6.2)
	Version        string
	Tags           []string
	Content        []byte // DESIGN.md text (raw body)
	PreviewDesktop string // legacy default desktop sha256 from POST /images
	PreviewMobile  string // legacy default mobile sha256 from POST /images
	Previews       []DesignPreviewInput
	ReviewRequired bool
	Official       bool // admin publishes default to official (§8.4)
}

type DesignPreviewInput struct {
	Theme  string `json:"theme"`
	Device string `json:"device"`
	SHA256 string `json:"sha256"`
}

// Publish runs the §10.3 write path in a single transaction: preview blob
// refs → designmd upsert → version insert (content+sha256) → latest
// pointer → tag association.
func (s *Designs) Publish(ctx context.Context, in DesignPublishInput) (*store.Designmd, *store.DesignmdVersion, error) {
	if !store.ValidateArtifactName(in.Name) {
		return nil, nil, fmt.Errorf("%w: name must match ^[a-z0-9](-?[a-z0-9])*$, ≤64", ErrInvalidInput)
	}
	if utf8.RuneCountInString(in.Summary) > 1024 {
		return nil, nil, fmt.Errorf("%w: summary too long", ErrInvalidInput)
	}
	if !semver.IsValid("v" + in.Version) {
		return nil, nil, fmt.Errorf("%w: version must be semver without v prefix", ErrInvalidInput)
	}
	for _, t := range in.Tags {
		if !store.ValidateTagName(t) {
			return nil, nil, fmt.Errorf("%w: invalid tag %q", ErrInvalidInput, t)
		}
	}
	if len(in.Content) == 0 || !utf8.Valid(in.Content) {
		return nil, nil, fmt.Errorf("%w: content must be non-empty UTF-8", ErrInvalidInput)
	}
	sum := sha256.Sum256(in.Content)
	contentSHA := hex.EncodeToString(sum[:])
	if in.Previews != nil && (in.PreviewDesktop != "" || in.PreviewMobile != "") {
		return nil, nil, fmt.Errorf("%w: cannot combine previews and legacy preview fields", ErrInvalidInput)
	}
	previews := in.Previews
	if in.PreviewDesktop != "" {
		previews = append(previews, DesignPreviewInput{"default", "desktop", in.PreviewDesktop})
	}
	if in.PreviewMobile != "" {
		previews = append(previews, DesignPreviewInput{"default", "mobile", in.PreviewMobile})
	}
	v := &store.DesignmdVersion{}
	slots := map[string]**string{
		"default/desktop": &v.PreviewDesktopSHA256, "default/mobile": &v.PreviewMobileSHA256,
		"light/desktop": &v.PreviewLightDesktopSHA256, "light/mobile": &v.PreviewLightMobileSHA256,
		"dark/desktop": &v.PreviewDarkDesktopSHA256, "dark/mobile": &v.PreviewDarkMobileSHA256,
	}
	seen := map[string]bool{}
	for _, p := range previews {
		key := p.Theme + "/" + p.Device
		_, valid := slots[key]
		_, hashErr := hex.DecodeString(p.SHA256)
		if !valid || seen[key] || len(p.SHA256) != 64 || hashErr != nil || p.SHA256 != strings.ToLower(p.SHA256) {
			return nil, nil, fmt.Errorf("%w: invalid or duplicate preview slot %q", ErrInvalidInput, key)
		}
		seen[key] = true
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// preview references must be pre-uploaded image blobs (§10.1)
	refPreview := func(sha string) (*string, error) {
		if sha == "" {
			return nil, nil
		}
		kind, _, err := s.blobs.Stat(ctx, tx, sha)
		if errors.Is(err, store.ErrNotFound) || (err == nil && kind != blob.KindImage) {
			return nil, fmt.Errorf("%w: preview must reference an uploaded image", ErrInvalidInput)
		}
		if err != nil {
			return nil, err
		}
		if err := s.blobs.AddRef(ctx, tx, sha, blob.KindImage); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, fmt.Errorf("%w: preview blob %s not found", ErrInvalidInput, sha)
			}
			return nil, err
		}
		return &sha, nil
	}
	for _, p := range previews {
		sha, err := refPreview(p.SHA256)
		if err != nil {
			return nil, nil, err
		}
		*slots[p.Theme+"/"+p.Device] = sha
	}

	designs := store.NewDesignmdStore(tx)
	versions := store.NewDesignmdVersionStore(tx)
	tags := store.NewTagStore(tx, s.dialect)

	now := store.Now()
	d, err := designs.GetByOwnerName(ctx, in.Owner.ID, in.Name)
	switch {
	case errors.Is(err, store.ErrNotFound):
		status := store.DesignStatusPublished
		if in.ReviewRequired {
			status = store.DesignStatusPending
		}
		d = &store.Designmd{
			ID: store.NewID(), OwnerID: in.Owner.ID, Name: in.Name,
			Official: in.Official,
			Status:   status, CreatedAt: now, UpdatedAt: now,
		}
		if err := designs.Create(ctx, d); err != nil {
			return nil, nil, err
		}
	case err != nil:
		return nil, nil, err
	}

	if _, err := versions.GetByDesignVersion(ctx, d.ID, in.Version); err == nil {
		return nil, nil, ErrVersionExists
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, nil, err
	}

	v.ID, v.DesignmdID, v.Version = store.NewID(), d.ID, in.Version
	v.Content, v.SHA256, v.CreatedAt = string(in.Content), contentSHA, now
	if err := versions.Create(ctx, v); err != nil {
		if store.IsUniqueViolation(err) {
			return nil, nil, ErrVersionExists
		}
		return nil, nil, err
	}
	if err := designs.UpdateAfterPublish(ctx, d.ID, v.ID, in.Summary, now); err != nil {
		return nil, nil, err
	}
	if err := tags.SetDesignTags(ctx, d.ID, in.Tags); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	d.Summary = in.Summary
	d.LatestVersionID = &v.ID
	d.UpdatedAt = now
	return d, v, nil
}

// GetDetail loads the detail read model by namespace (username) and name.
func (s *Designs) GetDetail(ctx context.Context, namespace, name string) (*DesignDetail, error) {
	return s.detailBy(ctx, `WHERE u.username = ? AND d.name = ?`, namespace, name)
}

// GetDetailByID loads the detail read model by designmd id.
func (s *Designs) GetDetailByID(ctx context.Context, id string) (*DesignDetail, error) {
	return s.detailBy(ctx, `WHERE d.id = ?`, id)
}

func (s *Designs) detailBy(ctx context.Context, cond string, args ...any) (*DesignDetail, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT d.id, d.owner_id, d.name, d.summary, d.official, d.status,
		        d.latest_version_id, d.download_count, d.created_at, d.updated_at,
		        u.username, u.nickname
		 FROM designmd d JOIN user u ON u.id = d.owner_id
		 `+cond, args...)
	det := &DesignDetail{DesignItem: DesignItem{Design: &store.Designmd{}}}
	err := row.Scan(&det.Design.ID, &det.Design.OwnerID, &det.Design.Name,
		&det.Design.Summary, &det.Design.Official, &det.Design.Status,
		&det.Design.LatestVersionID, &det.Design.DownloadCount,
		&det.Design.CreatedAt, &det.Design.UpdatedAt,
		&det.OwnerUsername, &det.OwnerNickname)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	det.Tags, err = store.NewTagStore(s.db, s.dialect).TagsForDesign(ctx, det.Design.ID)
	if err != nil {
		return nil, err
	}
	if det.Design.LatestVersionID != nil {
		lv, err := store.NewDesignmdVersionStore(s.db).GetByID(ctx, *det.Design.LatestVersionID)
		if err != nil {
			return nil, err
		}
		det.Latest = lv
		det.LatestVersion = lv.Version
		det.PreviewDesktopSHA256 = lv.CoverSHA256()
	}
	return det, nil
}

// List runs the §8.3 list query (symmetric with skills) plus the latest
// version's desktop preview for the card thumbnail.
func (s *Designs) List(ctx context.Context, f ListFilter) (items []DesignItem, total int, err error) {
	where := []string{"1=1"}
	args := []any{}
	if f.PublicOnly {
		where = append(where, `d.status = 'published'`)
	} else if f.Status != "" {
		where = append(where, `d.status = ?`)
		args = append(args, f.Status)
	}
	if f.OwnerID != "" {
		where = append(where, `d.owner_id = ?`)
		args = append(args, f.OwnerID)
	}
	if f.Official {
		where = append(where, `d.official = 1`)
	}
	if f.Query != "" {
		where = append(where, `(d.name LIKE ? OR d.summary LIKE ?)`)
		like := "%" + f.Query + "%"
		args = append(args, like, like)
	}
	if f.Tag != "" {
		where = append(where,
			`d.id IN (SELECT dt.designmd_id FROM designmd_tag dt JOIN tag t ON t.id = dt.tag_id WHERE t.name = ?)`)
		args = append(args, f.Tag)
	}
	cond := strings.Join(where, " AND ")

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM designmd d WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := `d.updated_at DESC`
	if f.Sort == "downloads" {
		order = `d.download_count DESC`
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	query := `
		SELECT d.id, d.owner_id, d.name, d.summary, d.official, d.status,
		       d.latest_version_id, d.download_count, d.created_at, d.updated_at,
		       u.username, u.nickname, COALESCE(lv.version, ''),
		       COALESCE(lv.preview_desktop_sha256, lv.preview_light_desktop_sha256,
		         lv.preview_dark_desktop_sha256, lv.preview_mobile_sha256,
		         lv.preview_light_mobile_sha256, lv.preview_dark_mobile_sha256)
		FROM designmd d
		JOIN user u ON u.id = d.owner_id
		LEFT JOIN designmd_version lv ON lv.id = d.latest_version_id
		WHERE ` + cond + `
		ORDER BY ` + order + `
		LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	ids := []string{}
	for rows.Next() {
		it := DesignItem{Design: &store.Designmd{}}
		if err := rows.Scan(&it.Design.ID, &it.Design.OwnerID, &it.Design.Name,
			&it.Design.Summary, &it.Design.Official, &it.Design.Status,
			&it.Design.LatestVersionID, &it.Design.DownloadCount,
			&it.Design.CreatedAt, &it.Design.UpdatedAt,
			&it.OwnerUsername, &it.OwnerNickname, &it.LatestVersion,
			&it.PreviewDesktopSHA256); err != nil {
			return nil, 0, err
		}
		ids = append(ids, it.Design.ID)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	tagMap, err := store.NewTagStore(s.db, s.dialect).TagsByDesignIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].Tags = tagMap[items[i].Design.ID]
	}
	return items, total, nil
}

// ListVersions returns all versions of a designmd, highest semver first.
func (s *Designs) ListVersions(ctx context.Context, designID string) ([]*store.DesignmdVersion, error) {
	versions, err := store.NewDesignmdVersionStore(s.db).ListByDesign(ctx, designID)
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(versions); i++ {
		for j := i; j > 0 && semver.Compare("v"+versions[j-1].Version, "v"+versions[j].Version) < 0; j-- {
			versions[j-1], versions[j] = versions[j], versions[j-1]
		}
	}
	return versions, nil
}

func (s *Designs) GetVersion(ctx context.Context, designID, version string) (*store.DesignmdVersion, error) {
	return store.NewDesignmdVersionStore(s.db).GetByDesignVersion(ctx, designID, version)
}

func (s *Designs) getDesign(ctx context.Context, id string) (*store.Designmd, error) {
	return store.NewDesignmdStore(s.db).GetByID(ctx, id)
}

func canManageDesign(actor *store.User, d *store.Designmd) error {
	if actor.Role == store.RoleAdmin || actor.ID == d.OwnerID {
		return nil
	}
	return ErrForbidden
}

// Update edits summary and/or tags (§8.4: designmd 还可改 summary).
// A nil pointer/slice leaves the field unchanged.
func (s *Designs) Update(ctx context.Context, actor *store.User, designID string, summary *string, tags []string) error {
	d, err := s.getDesign(ctx, designID)
	if err != nil {
		return err
	}
	if err := canManageDesign(actor, d); err != nil {
		return err
	}
	if summary != nil {
		if utf8.RuneCountInString(*summary) > 1024 {
			return fmt.Errorf("%w: summary too long", ErrInvalidInput)
		}
		if err := store.NewDesignmdStore(s.db).UpdateSummary(ctx, designID, *summary, store.Now()); err != nil {
			return err
		}
	}
	if tags != nil {
		for _, t := range tags {
			if !store.ValidateTagName(t) {
				return fmt.Errorf("%w: invalid tag %q", ErrInvalidInput, t)
			}
		}
		if err := store.NewTagStore(s.db, s.dialect).SetDesignTags(ctx, designID, tags); err != nil {
			return err
		}
	}
	return nil
}

// SetStatus handles takedown/restore (§8.4, symmetric with skills).
func (s *Designs) SetStatus(ctx context.Context, actor *store.User, designID, action string) (*store.Designmd, error) {
	d, err := s.getDesign(ctx, designID)
	if err != nil {
		return nil, err
	}
	if err := canManageDesign(actor, d); err != nil {
		return nil, err
	}
	var status string
	switch action {
	case "takedown":
		if d.Status == store.DesignStatusPending {
			return nil, fmt.Errorf("%w: pending design cannot be taken down", ErrInvalidInput)
		}
		status = store.DesignStatusTakenDown
	case "restore":
		if d.Status != store.DesignStatusTakenDown {
			return nil, fmt.Errorf("%w: design is not taken down", ErrInvalidInput)
		}
		status = store.DesignStatusPublished
	default:
		return nil, fmt.Errorf("%w: unknown action %q", ErrInvalidInput, action)
	}
	if err := store.NewDesignmdStore(s.db).UpdateStatus(ctx, designID, status, store.Now()); err != nil {
		return nil, err
	}
	d.Status = status
	return d, nil
}

// Delete removes a designmd: versions cascade in DB; preview blob refs of
// every version are released in the same transaction (§8.4, §6.3).
func (s *Designs) Delete(ctx context.Context, actor *store.User, designID string) error {
	d, err := s.getDesign(ctx, designID)
	if err != nil {
		return err
	}
	if err := canManageDesign(actor, d); err != nil {
		return err
	}

	versions, err := store.NewDesignmdVersionStore(s.db).ListByDesign(ctx, designID)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := store.NewDesignmdStore(tx).Delete(ctx, designID); err != nil {
		return err
	}
	for _, v := range versions {
		for _, p := range v.Previews() {
			if err := s.blobs.Release(ctx, tx, *p.SHA256); err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}
		}
	}
	return tx.Commit()
}

// IncrDownload bumps the download counter (§6.3).
func (s *Designs) IncrDownload(ctx context.Context, designID string) {
	_ = store.NewDesignmdStore(s.db).IncrDownload(ctx, designID)
}
