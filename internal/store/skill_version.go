package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SkillVersion keeps the JSON columns (harnesses/permissions/frontmatter)
// as raw JSON text; marshaling lives in the hub/skillpkg layers.
type SkillVersion struct {
	ID            string
	SkillID       string
	Version       string // semver, no v prefix
	Description   string
	License       string
	Compatibility string
	Author        string
	Harnesses     string // JSON array; [] means generic
	Permissions   string // JSON array of {tool,risk,note}
	Frontmatter   string // JSON object, raw passthrough
	FileCount     int
	SHA256        string
	Size          int64
	Filename      string
	CreatedAt     time.Time
}

type SkillVersionStore struct {
	db DBTX
}

func NewSkillVersionStore(db DBTX) *SkillVersionStore { return &SkillVersionStore{db: db} }

const skillVersionColumns = `id, skill_id, version, description, license, compatibility, author, harnesses, permissions, frontmatter, file_count, sha256, size, filename, created_at`

func (s *SkillVersionStore) Create(ctx context.Context, v *SkillVersion) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO skill_version (`+skillVersionColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.SkillID, v.Version, v.Description, v.License, v.Compatibility,
		v.Author, v.Harnesses, v.Permissions, v.Frontmatter, v.FileCount,
		v.SHA256, v.Size, v.Filename, v.CreatedAt)
	return err
}

func scanSkillVersion(row interface{ Scan(...any) error }) (*SkillVersion, error) {
	var v SkillVersion
	err := row.Scan(&v.ID, &v.SkillID, &v.Version, &v.Description, &v.License,
		&v.Compatibility, &v.Author, &v.Harnesses, &v.Permissions, &v.Frontmatter,
		&v.FileCount, &v.SHA256, &v.Size, &v.Filename, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *SkillVersionStore) GetByID(ctx context.Context, id string) (*SkillVersion, error) {
	return scanSkillVersion(s.db.QueryRowContext(ctx,
		`SELECT `+skillVersionColumns+` FROM skill_version WHERE id = ?`, id))
}

// UpdateDescription edits one version's description (post-publish edit).
func (s *SkillVersionStore) UpdateDescription(ctx context.Context, id, description string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE skill_version SET description = ? WHERE id = ?`, description, id)
	return err
}

func (s *SkillVersionStore) GetBySkillVersion(ctx context.Context, skillID, version string) (*SkillVersion, error) {
	return scanSkillVersion(s.db.QueryRowContext(ctx,
		`SELECT `+skillVersionColumns+` FROM skill_version WHERE skill_id = ? AND version = ?`,
		skillID, version))
}

// ListBySkill returns versions newest first (by creation time).
func (s *SkillVersionStore) ListBySkill(ctx context.Context, skillID string) ([]*SkillVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+skillVersionColumns+` FROM skill_version WHERE skill_id = ? ORDER BY created_at DESC`,
		skillID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*SkillVersion
	for rows.Next() {
		v, err := scanSkillVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
