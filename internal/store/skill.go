package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Skill statuses (VARCHAR(16) + app-level validation, design §6.1).
const (
	SkillStatusPending   = "pending"
	SkillStatusPublished = "published"
	SkillStatusTakenDown = "taken_down"
)

type Skill struct {
	ID              string
	OwnerID         string
	Name            string
	Summary         string
	Official        bool
	Status          string
	LatestVersionID *string
	DownloadCount   int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SkillStore struct {
	db DBTX
}

func NewSkillStore(db DBTX) *SkillStore { return &SkillStore{db: db} }

const skillColumns = `id, owner_id, name, summary, official, status, latest_version_id, download_count, created_at, updated_at`

func (s *SkillStore) Create(ctx context.Context, sk *Skill) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO skill (`+skillColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sk.ID, sk.OwnerID, sk.Name, sk.Summary, sk.Official, sk.Status,
		sk.LatestVersionID, sk.DownloadCount, sk.CreatedAt, sk.UpdatedAt)
	return err
}

func scanSkill(row interface{ Scan(...any) error }) (*Skill, error) {
	var sk Skill
	err := row.Scan(&sk.ID, &sk.OwnerID, &sk.Name, &sk.Summary, &sk.Official,
		&sk.Status, &sk.LatestVersionID, &sk.DownloadCount, &sk.CreatedAt, &sk.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sk, nil
}

func (s *SkillStore) GetByID(ctx context.Context, id string) (*Skill, error) {
	return scanSkill(s.db.QueryRowContext(ctx,
		`SELECT `+skillColumns+` FROM skill WHERE id = ?`, id))
}

func (s *SkillStore) GetByOwnerName(ctx context.Context, ownerID, name string) (*Skill, error) {
	return scanSkill(s.db.QueryRowContext(ctx,
		`SELECT `+skillColumns+` FROM skill WHERE owner_id = ? AND name = ?`, ownerID, name))
}

// UpdateAfterPublish moves the latest pointer and refreshes the summary.
func (s *SkillStore) UpdateAfterPublish(ctx context.Context, id, latestVersionID, summary string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE skill SET latest_version_id = ?, summary = ?, updated_at = ? WHERE id = ?`,
		latestVersionID, summary, updatedAt, id)
	return err
}

func (s *SkillStore) UpdateSummary(ctx context.Context, id, summary string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE skill SET summary = ?, updated_at = ? WHERE id = ?`, summary, updatedAt, id)
	return err
}

func (s *SkillStore) UpdateStatus(ctx context.Context, id, status string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE skill SET status = ?, updated_at = ? WHERE id = ?`, status, updatedAt, id)
	return err
}

func (s *SkillStore) UpdateOfficial(ctx context.Context, id string, official bool, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE skill SET official = ?, updated_at = ? WHERE id = ?`, official, updatedAt, id)
	return err
}

func (s *SkillStore) IncrDownload(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE skill SET download_count = download_count + 1 WHERE id = ?`, id)
	return err
}

// Delete removes the skill row; versions and tag links cascade.
func (s *SkillStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM skill WHERE id = ?`, id)
	return err
}
