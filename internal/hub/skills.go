// Package hub is the business orchestration layer: publish, versions,
// takedown/restore/delete, tag editing (design §3.2). It owns no HTTP
// concerns; handlers live in the server package.
package hub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/semver"

	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/skillpkg"
	"github.com/ravenmk2/rune-market/internal/store"
)

// Sentinel errors mapped to HTTP codes by handlers.
var (
	ErrNotFound      = store.ErrNotFound
	ErrForbidden     = errors.New("hub: forbidden")
	ErrVersionExists = errors.New("hub: version already exists")
	ErrInvalidInput  = errors.New("hub: invalid input")
)

// SkillItem is the list-card read model (contract item shape).
type SkillItem struct {
	Skill         *store.Skill
	OwnerUsername string
	OwnerNickname string
	Tags          []string
	LatestVersion string
	IconExt       string // stored extension of the icon image ("png"/"jpg"), "" if none
}

// SkillDetail is the detail read model: item fields plus the latest
// version's full metadata.
type SkillDetail struct {
	SkillItem
	Latest *store.SkillVersion
}

// ListFilter carries the §8.3 list query parameters.
type ListFilter struct {
	Query      string
	Tag        string
	Official   bool
	Status     string // "" = any; a status value for admin views
	Sort       string // "" (updated) | "downloads"
	Page       int
	PageSize   int
	PublicOnly bool   // restrict to status=published
	OwnerID    string // restrict to one owner (mine)
}

type Skills struct {
	db      *sql.DB
	dialect string
	blobs   *blob.Storage
}

func NewSkills(db *sql.DB, dialect string, blobs *blob.Storage) *Skills {
	return &Skills{db: db, dialect: dialect, blobs: blobs}
}

// DB exposes the connection pool for handlers that need direct access.
func (s *Skills) DB() *sql.DB { return s.db }

// PublishInput is one publish request (§8.4): the archive was pre-uploaded
// via POST /archives and is referenced by its blob sha256.
type PublishInput struct {
	Owner          *store.User
	Version        string   // semver, no v prefix, from the form
	Tags           []string // full replacement tag set
	Description    string   // optional override for the package meta description
	ArchiveSHA256  string   // archive blob from POST /archives
	IconSHA256     string   // optional icon image blob from POST /images
	ReviewRequired bool     // artifact_review=required
	Official       bool     // admin publishes default to official (§8.4)
}

// Publish runs §9.8 in a single DB transaction over a pre-uploaded archive:
// resolve the archive blob (must exist, kind=archive) → re-inspect it from
// the blob path (the server trusts the stored bytes, not the client) →
// AddRef the archive → skill upsert → version insert → latest pointer →
// tag association → optional icon binding (AddRef image, swap old ref).
func (s *Skills) Publish(ctx context.Context, in PublishInput) (*store.Skill, *store.SkillVersion, error) {
	if !semver.IsValid("v" + in.Version) {
		return nil, nil, fmt.Errorf("%w: version must be semver without v prefix", ErrInvalidInput)
	}
	for _, t := range in.Tags {
		if !store.ValidateTagName(t) {
			return nil, nil, fmt.Errorf("%w: invalid tag %q", ErrInvalidInput, t)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	kind, size, err := s.blobs.Stat(ctx, tx, in.ArchiveSHA256)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, fmt.Errorf("%w: archive blob %s not found", ErrInvalidInput, in.ArchiveSHA256)
	}
	if err != nil {
		return nil, nil, err
	}
	if kind != blob.KindArchive {
		return nil, nil, fmt.Errorf("%w: blob %s is not an archive", ErrInvalidInput, in.ArchiveSHA256)
	}
	archivePath, err := s.blobs.Path(blob.KindArchive, in.ArchiveSHA256, "")
	if err != nil {
		return nil, nil, err
	}
	pkg, err := skillpkg.Inspect(archivePath, size, in.ArchiveSHA256)
	if err != nil {
		return nil, nil, err
	}
	meta := pkg.Report.Metadata
	if meta == nil || pkg.Report.HasErrors() {
		return nil, nil, fmt.Errorf("%w: package has validation errors", ErrInvalidInput)
	}
	author := meta.Author
	if author == "" {
		author = in.Owner.Nickname // §9.5: publisher nickname fallback
	}

	// a non-empty form description replaces the package meta description;
	// it feeds both skill.summary and this version's description (§8.4)
	description := meta.Description
	if d := strings.TrimSpace(in.Description); d != "" {
		if utf8.RuneCountInString(d) > 1024 {
			return nil, nil, fmt.Errorf("%w: description must be at most 1024 characters", ErrInvalidInput)
		}
		description = d
	}

	harnesses, err := jsonArray(meta.Harnesses)
	if err != nil {
		return nil, nil, err
	}
	permissions, err := jsonArray(meta.Permissions)
	if err != nil {
		return nil, nil, err
	}

	// binding reference: the upload already holds one (§10.3 semantics)
	if err := s.blobs.AddRef(ctx, tx, in.ArchiveSHA256, blob.KindArchive); err != nil {
		return nil, nil, err
	}

	var iconSHA *string
	if in.IconSHA256 != "" {
		if err := s.bindIcon(ctx, tx, in.IconSHA256); err != nil {
			return nil, nil, err
		}
		iconSHA = &in.IconSHA256
	}

	skills := store.NewSkillStore(tx)
	versions := store.NewSkillVersionStore(tx)
	tags := store.NewTagStore(tx, s.dialect)

	now := store.Now()
	sk, err := skills.GetByOwnerName(ctx, in.Owner.ID, meta.Name)
	switch {
	case errors.Is(err, store.ErrNotFound):
		status := store.SkillStatusPublished
		if in.ReviewRequired {
			status = store.SkillStatusPending
		}
		sk = &store.Skill{
			ID: store.NewID(), OwnerID: in.Owner.ID, Name: meta.Name,
			Official: in.Official, IconSHA256: iconSHA,
			Status: status, CreatedAt: now, UpdatedAt: now,
		}
		if err := skills.Create(ctx, sk); err != nil {
			return nil, nil, err
		}
	case err != nil:
		return nil, nil, err
	}

	if _, err := versions.GetBySkillVersion(ctx, sk.ID, in.Version); err == nil {
		return nil, nil, ErrVersionExists
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, nil, err
	}

	v := &store.SkillVersion{
		ID: store.NewID(), SkillID: sk.ID, Version: in.Version,
		Description: description, License: meta.License,
		Compatibility: meta.Compatibility, Author: author,
		Harnesses: harnesses, Permissions: permissions,
		Frontmatter: pkg.Frontmatter,
		FileCount:   meta.FileCount,
		SHA256:      in.ArchiveSHA256, Size: size,
		Filename:  fmt.Sprintf("%s-%s.%s", meta.Name, in.Version, pkg.Format),
		CreatedAt: now,
	}
	if err := versions.Create(ctx, v); err != nil {
		if store.IsUniqueViolation(err) {
			return nil, nil, ErrVersionExists
		}
		return nil, nil, err
	}
	if err := skills.UpdateAfterPublish(ctx, sk.ID, v.ID, description, now); err != nil {
		return nil, nil, err
	}
	if err := tags.SetSkillTags(ctx, sk.ID, in.Tags); err != nil {
		return nil, nil, err
	}
	// publishing with an icon over an existing skill swaps the icon reference
	// (a freshly created row already carries iconSHA)
	if iconSHA != nil && sk.IconSHA256 == nil {
		if err := skills.UpdateIcon(ctx, sk.ID, iconSHA, now); err != nil {
			return nil, nil, err
		}
	} else if iconSHA != nil && *sk.IconSHA256 != *iconSHA {
		old := *sk.IconSHA256
		if err := skills.UpdateIcon(ctx, sk.ID, iconSHA, now); err != nil {
			return nil, nil, err
		}
		if err := s.blobs.Release(ctx, tx, old); err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	if iconSHA != nil {
		sk.IconSHA256 = iconSHA
	}
	sk.Summary = description
	sk.LatestVersionID = &v.ID
	sk.UpdatedAt = now
	return sk, v, nil
}

// bindIcon verifies the icon blob is an uploaded image and takes a binding
// reference on it.
func (s *Skills) bindIcon(ctx context.Context, q store.DBTX, sha string) error {
	kind, _, err := s.blobs.Stat(ctx, q, sha)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("%w: icon blob %s not found", ErrInvalidInput, sha)
	}
	if err != nil {
		return err
	}
	if kind != blob.KindImage {
		return fmt.Errorf("%w: icon blob %s is not an image", ErrInvalidInput, sha)
	}
	return s.blobs.AddRef(ctx, q, sha, blob.KindImage)
}

// GetDetail loads the detail read model by namespace (username) and name.
func (s *Skills) GetDetail(ctx context.Context, namespace, name string) (*SkillDetail, error) {
	return s.detailBy(ctx, `WHERE u.username = ? AND s.name = ?`, namespace, name)
}

// GetDetailByID loads the detail read model by skill id.
func (s *Skills) GetDetailByID(ctx context.Context, id string) (*SkillDetail, error) {
	return s.detailBy(ctx, `WHERE s.id = ?`, id)
}

func (s *Skills) detailBy(ctx context.Context, cond string, args ...any) (*SkillDetail, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT s.id, s.owner_id, s.name, s.summary, s.official, s.status,
		        s.latest_version_id, s.icon_sha256, s.download_count, s.created_at, s.updated_at,
		        u.username, u.nickname
		 FROM skill s JOIN user u ON u.id = s.owner_id
		 `+cond, args...)
	d := &SkillDetail{SkillItem: SkillItem{Skill: &store.Skill{}}}
	err := row.Scan(&d.Skill.ID, &d.Skill.OwnerID, &d.Skill.Name, &d.Skill.Summary,
		&d.Skill.Official, &d.Skill.Status, &d.Skill.LatestVersionID, &d.Skill.IconSHA256,
		&d.Skill.DownloadCount, &d.Skill.CreatedAt, &d.Skill.UpdatedAt,
		&d.OwnerUsername, &d.OwnerNickname)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	exts, err := s.iconExts(ctx, []SkillItem{d.SkillItem})
	if err != nil {
		return nil, err
	}
	if d.Skill.IconSHA256 != nil {
		d.IconExt = exts[*d.Skill.IconSHA256]
	}

	tags := store.NewTagStore(s.db, s.dialect)
	d.Tags, err = tags.TagsForSkill(ctx, d.Skill.ID)
	if err != nil {
		return nil, err
	}
	if d.Skill.LatestVersionID != nil {
		versions := store.NewSkillVersionStore(s.db)
		lv, err := versions.GetByID(ctx, *d.Skill.LatestVersionID)
		if err != nil {
			return nil, err
		}
		d.Latest = lv
		d.LatestVersion = lv.Version
	}
	return d, nil
}

// List runs the §8.3 list query and returns items plus the total count.
func (s *Skills) List(ctx context.Context, f ListFilter) (items []SkillItem, total int, err error) {
	where := []string{"1=1"}
	args := []any{}
	if f.PublicOnly {
		where = append(where, `s.status = 'published'`)
	} else if f.Status != "" {
		where = append(where, `s.status = ?`)
		args = append(args, f.Status)
	}
	if f.OwnerID != "" {
		where = append(where, `s.owner_id = ?`)
		args = append(args, f.OwnerID)
	}
	if f.Official {
		where = append(where, `s.official = 1`)
	}
	if f.Query != "" {
		where = append(where, `(s.name LIKE ? OR s.summary LIKE ?)`)
		like := "%" + f.Query + "%"
		args = append(args, like, like)
	}
	if f.Tag != "" {
		where = append(where,
			`s.id IN (SELECT st.skill_id FROM skill_tag st JOIN tag t ON t.id = st.tag_id WHERE t.name = ?)`)
		args = append(args, f.Tag)
	}
	cond := strings.Join(where, " AND ")

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skill s WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := `s.updated_at DESC`
	if f.Sort == "downloads" {
		order = `s.download_count DESC`
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
		SELECT s.id, s.owner_id, s.name, s.summary, s.official, s.status,
		       s.latest_version_id, s.icon_sha256, s.download_count, s.created_at, s.updated_at,
		       u.username, u.nickname, COALESCE(lv.version, '')
		FROM skill s
		JOIN user u ON u.id = s.owner_id
		LEFT JOIN skill_version lv ON lv.id = s.latest_version_id
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
		it := SkillItem{Skill: &store.Skill{}}
		if err := rows.Scan(&it.Skill.ID, &it.Skill.OwnerID, &it.Skill.Name,
			&it.Skill.Summary, &it.Skill.Official, &it.Skill.Status,
			&it.Skill.LatestVersionID, &it.Skill.IconSHA256, &it.Skill.DownloadCount,
			&it.Skill.CreatedAt, &it.Skill.UpdatedAt,
			&it.OwnerUsername, &it.OwnerNickname, &it.LatestVersion); err != nil {
			return nil, 0, err
		}
		ids = append(ids, it.Skill.ID)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	tagMap, err := store.NewTagStore(s.db, s.dialect).TagsBySkillIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	exts, err := s.iconExts(ctx, items)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].Tags = tagMap[items[i].Skill.ID]
		if items[i].Skill.IconSHA256 != nil {
			items[i].IconExt = exts[*items[i].Skill.IconSHA256]
		}
	}
	return items, total, nil
}

// iconExts batch-resolves the stored extensions of the items' icon images
// (one query; keyed by sha256).
func (s *Skills) iconExts(ctx context.Context, items []SkillItem) (map[string]string, error) {
	var shas []string
	for _, it := range items {
		if it.Skill.IconSHA256 != nil {
			shas = append(shas, *it.Skill.IconSHA256)
		}
	}
	return s.blobs.ImageExts(ctx, s.db, shas)
}

// ListVersions returns all versions of a skill, highest semver first.
func (s *Skills) ListVersions(ctx context.Context, skillID string) ([]*store.SkillVersion, error) {
	versions, err := store.NewSkillVersionStore(s.db).ListBySkill(ctx, skillID)
	if err != nil {
		return nil, err
	}
	// semver descending (§6.3 ordering rule)
	for i := 1; i < len(versions); i++ {
		for j := i; j > 0 && semver.Compare("v"+versions[j-1].Version, "v"+versions[j].Version) < 0; j-- {
			versions[j-1], versions[j] = versions[j], versions[j-1]
		}
	}
	return versions, nil
}

func (s *Skills) GetVersion(ctx context.Context, skillID, version string) (*store.SkillVersion, error) {
	return store.NewSkillVersionStore(s.db).GetBySkillVersion(ctx, skillID, version)
}

// canManage enforces owner-or-admin (§7 resource-level checks).
func canManage(actor *store.User, sk *store.Skill) error {
	if actor.Role == store.RoleAdmin || actor.ID == sk.OwnerID {
		return nil
	}
	return ErrForbidden
}

func (s *Skills) getSkill(ctx context.Context, id string) (*store.Skill, error) {
	return store.NewSkillStore(s.db).GetByID(ctx, id)
}

// Update edits summary, current-version description, tags and/or icon
// (§8.4 PUT /skills/{id}). Nil pointer/slice leaves the field unchanged;
// icon "" clears the icon (releasing its blob ref), otherwise it must be an
// uploaded image blob sha256.
func (s *Skills) Update(ctx context.Context, actor *store.User, skillID string, summary, description *string, tags []string, icon *string) error {
	sk, err := s.getSkill(ctx, skillID)
	if err != nil {
		return err
	}
	if err := canManage(actor, sk); err != nil {
		return err
	}
	if summary != nil {
		if utf8.RuneCountInString(*summary) > 1024 {
			return fmt.Errorf("%w: summary too long", ErrInvalidInput)
		}
		if err := store.NewSkillStore(s.db).UpdateSummary(ctx, skillID, *summary, store.Now()); err != nil {
			return err
		}
	}
	if description != nil {
		if utf8.RuneCountInString(*description) > 1024 {
			return fmt.Errorf("%w: description too long", ErrInvalidInput)
		}
		if sk.LatestVersionID == nil {
			return fmt.Errorf("%w: skill has no version", ErrInvalidInput)
		}
		if err := store.NewSkillVersionStore(s.db).UpdateDescription(ctx, *sk.LatestVersionID, *description); err != nil {
			return err
		}
	}
	if tags != nil {
		for _, t := range tags {
			if !store.ValidateTagName(t) {
				return fmt.Errorf("%w: invalid tag %q", ErrInvalidInput, t)
			}
		}
		if err := store.NewTagStore(s.db, s.dialect).SetSkillTags(ctx, skillID, tags); err != nil {
			return err
		}
	}
	if icon != nil {
		return s.updateIcon(ctx, sk, icon)
	}
	return nil
}

// updateIcon swaps or clears the skill icon with ref accounting in one
// transaction: AddRef the new image before releasing the old one.
func (s *Skills) updateIcon(ctx context.Context, sk *store.Skill, icon *string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := store.Now()
	if *icon == "" {
		if sk.IconSHA256 == nil {
			return nil
		}
		if err := store.NewSkillStore(tx).UpdateIcon(ctx, sk.ID, nil, now); err != nil {
			return err
		}
		if err := s.blobs.Release(ctx, tx, *sk.IconSHA256); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		return tx.Commit()
	}
	if sk.IconSHA256 != nil && *sk.IconSHA256 == *icon {
		return nil // unchanged
	}
	if err := s.bindIcon(ctx, tx, *icon); err != nil {
		return err
	}
	if err := store.NewSkillStore(tx).UpdateIcon(ctx, sk.ID, icon, now); err != nil {
		return err
	}
	if sk.IconSHA256 != nil {
		if err := s.blobs.Release(ctx, tx, *sk.IconSHA256); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	return tx.Commit()
}

// SetStatus handles takedown/restore (§8.4).
func (s *Skills) SetStatus(ctx context.Context, actor *store.User, skillID, action string) (*store.Skill, error) {
	sk, err := s.getSkill(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if err := canManage(actor, sk); err != nil {
		return nil, err
	}
	var status string
	switch action {
	case "takedown":
		if sk.Status == store.SkillStatusPending {
			return nil, fmt.Errorf("%w: pending skill cannot be taken down", ErrInvalidInput)
		}
		status = store.SkillStatusTakenDown
	case "restore":
		if sk.Status != store.SkillStatusTakenDown {
			return nil, fmt.Errorf("%w: skill is not taken down", ErrInvalidInput)
		}
		status = store.SkillStatusPublished
	default:
		return nil, fmt.Errorf("%w: unknown action %q", ErrInvalidInput, action)
	}
	if err := store.NewSkillStore(s.db).UpdateStatus(ctx, skillID, status, store.Now()); err != nil {
		return nil, err
	}
	sk.Status = status
	return sk, nil
}

// Delete removes a skill: versions cascade in DB, blob refs are released
// in the same transaction (§8.4, §6.3).
func (s *Skills) Delete(ctx context.Context, actor *store.User, skillID string) error {
	sk, err := s.getSkill(ctx, skillID)
	if err != nil {
		return err
	}
	if err := canManage(actor, sk); err != nil {
		return err
	}

	versions, err := store.NewSkillVersionStore(s.db).ListBySkill(ctx, skillID)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := store.NewSkillStore(tx).Delete(ctx, skillID); err != nil {
		return err
	}
	for _, v := range versions {
		if err := s.blobs.Release(ctx, tx, v.SHA256); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	if sk.IconSHA256 != nil {
		if err := s.blobs.Release(ctx, tx, *sk.IconSHA256); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	return tx.Commit()
}

// DownloadTarget carries everything the download handler needs.
type DownloadTarget struct {
	SkillID  string
	SHA256   string
	Filename string
	Size     int64
}

// ResolveDownload finds the blob for one version (§8.3 download).
func (s *Skills) ResolveDownload(ctx context.Context, namespace, name, version string) (*DownloadTarget, error) {
	d, err := s.GetDetail(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	v, err := s.GetVersion(ctx, d.Skill.ID, version)
	if err != nil {
		return nil, err
	}
	return &DownloadTarget{
		SkillID: d.Skill.ID, SHA256: v.SHA256, Filename: v.Filename, Size: v.Size,
	}, nil
}

// IncrDownload bumps the download counter (§6.3: approximate is fine).
func (s *Skills) IncrDownload(ctx context.Context, skillID string) {
	_ = store.NewSkillStore(s.db).IncrDownload(ctx, skillID)
}

func jsonArray(v any) (string, error) {
	b, err := jsonMarshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
