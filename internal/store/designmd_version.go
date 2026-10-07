package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// DesignmdVersion stores the DESIGN.md content directly in the DB (§6
// decision 5); sha256 is the content hash and does not reference blob.
// Preview sha columns reference blob(sha256) with kind=image.
type DesignmdVersion struct {
	ID                   string
	DesignmdID           string
	Version              string
	Content              string
	SHA256               string
	PreviewDesktopSHA256 *string
	PreviewMobileSHA256  *string
	CreatedAt            time.Time
}

type DesignmdVersionStore struct {
	db DBTX
}

func NewDesignmdVersionStore(db DBTX) *DesignmdVersionStore {
	return &DesignmdVersionStore{db: db}
}

const designmdVersionColumns = `id, designmd_id, version, content, sha256, preview_desktop_sha256, preview_mobile_sha256, created_at`

func (s *DesignmdVersionStore) Create(ctx context.Context, v *DesignmdVersion) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO designmd_version (`+designmdVersionColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.DesignmdID, v.Version, v.Content, v.SHA256,
		v.PreviewDesktopSHA256, v.PreviewMobileSHA256, v.CreatedAt)
	return err
}

func scanDesignmdVersion(row interface{ Scan(...any) error }) (*DesignmdVersion, error) {
	var v DesignmdVersion
	err := row.Scan(&v.ID, &v.DesignmdID, &v.Version, &v.Content, &v.SHA256,
		&v.PreviewDesktopSHA256, &v.PreviewMobileSHA256, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ScanPreview is used by list/detail queries that only need preview hashes.
func (s *DesignmdVersionStore) GetByID(ctx context.Context, id string) (*DesignmdVersion, error) {
	return scanDesignmdVersion(s.db.QueryRowContext(ctx,
		`SELECT `+designmdVersionColumns+` FROM designmd_version WHERE id = ?`, id))
}

func (s *DesignmdVersionStore) GetByDesignVersion(ctx context.Context, designID, version string) (*DesignmdVersion, error) {
	return scanDesignmdVersion(s.db.QueryRowContext(ctx,
		`SELECT `+designmdVersionColumns+` FROM designmd_version WHERE designmd_id = ? AND version = ?`,
		designID, version))
}

// ListByDesign returns versions newest first (by creation time).
func (s *DesignmdVersionStore) ListByDesign(ctx context.Context, designID string) ([]*DesignmdVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+designmdVersionColumns+` FROM designmd_version WHERE designmd_id = ? ORDER BY created_at DESC`,
		designID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*DesignmdVersion
	for rows.Next() {
		v, err := scanDesignmdVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
