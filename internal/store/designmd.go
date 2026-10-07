package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Design statuses share the skill state machine (§6.2).
const (
	DesignStatusPending   = SkillStatusPending
	DesignStatusPublished = SkillStatusPublished
	DesignStatusTakenDown = SkillStatusTakenDown
)

type Designmd struct {
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

type DesignmdStore struct {
	db DBTX
}

func NewDesignmdStore(db DBTX) *DesignmdStore { return &DesignmdStore{db: db} }

const designmdColumns = `id, owner_id, name, summary, official, status, latest_version_id, download_count, created_at, updated_at`

func (s *DesignmdStore) Create(ctx context.Context, d *Designmd) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO designmd (`+designmdColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.OwnerID, d.Name, d.Summary, d.Official, d.Status,
		d.LatestVersionID, d.DownloadCount, d.CreatedAt, d.UpdatedAt)
	return err
}

func scanDesignmd(row interface{ Scan(...any) error }) (*Designmd, error) {
	var d Designmd
	err := row.Scan(&d.ID, &d.OwnerID, &d.Name, &d.Summary, &d.Official,
		&d.Status, &d.LatestVersionID, &d.DownloadCount, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *DesignmdStore) GetByID(ctx context.Context, id string) (*Designmd, error) {
	return scanDesignmd(s.db.QueryRowContext(ctx,
		`SELECT `+designmdColumns+` FROM designmd WHERE id = ?`, id))
}

func (s *DesignmdStore) GetByOwnerName(ctx context.Context, ownerID, name string) (*Designmd, error) {
	return scanDesignmd(s.db.QueryRowContext(ctx,
		`SELECT `+designmdColumns+` FROM designmd WHERE owner_id = ? AND name = ?`, ownerID, name))
}

func (s *DesignmdStore) UpdateAfterPublish(ctx context.Context, id, latestVersionID, summary string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE designmd SET latest_version_id = ?, summary = ?, updated_at = ? WHERE id = ?`,
		latestVersionID, summary, updatedAt, id)
	return err
}

func (s *DesignmdStore) UpdateSummary(ctx context.Context, id, summary string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE designmd SET summary = ?, updated_at = ? WHERE id = ?`, summary, updatedAt, id)
	return err
}

func (s *DesignmdStore) UpdateStatus(ctx context.Context, id, status string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE designmd SET status = ?, updated_at = ? WHERE id = ?`, status, updatedAt, id)
	return err
}

func (s *DesignmdStore) UpdateOfficial(ctx context.Context, id string, official bool, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE designmd SET official = ?, updated_at = ? WHERE id = ?`, official, updatedAt, id)
	return err
}

func (s *DesignmdStore) IncrDownload(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE designmd SET download_count = download_count + 1 WHERE id = ?`, id)
	return err
}

// Delete removes the designmd row; versions and tag links cascade.
func (s *DesignmdStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM designmd WHERE id = ?`, id)
	return err
}
