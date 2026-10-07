package store

import "context"

// Overview is the admin dashboard statistics bundle (§8.5).
type Overview struct {
	Users          int
	Skills         int
	Designs        int
	StorageBytes   int64
	PendingUsers   int
	PendingSkills  int
	PendingDesigns int
}

type StatsStore struct {
	db DBTX
}

func NewStatsStore(db DBTX) *StatsStore { return &StatsStore{db: db} }

// Overview gathers the dashboard counters. Avatar directory scanning is
// added in M5 (§6.3); storage currently covers the blob table only.
func (s *StatsStore) Overview(ctx context.Context) (*Overview, error) {
	var o Overview
	queries := []struct {
		dst  *int
		sql  string
		args []any
	}{
		{&o.Users, `SELECT COUNT(*) FROM user`, nil},
		{&o.Skills, `SELECT COUNT(*) FROM skill`, nil},
		{&o.Designs, `SELECT COUNT(*) FROM designmd`, nil},
		{&o.PendingUsers, `SELECT COUNT(*) FROM user WHERE status = ?`, []any{StatusPending}},
		{&o.PendingSkills, `SELECT COUNT(*) FROM skill WHERE status = ?`, []any{SkillStatusPending}},
		{&o.PendingDesigns, `SELECT COUNT(*) FROM designmd WHERE status = ?`, []any{DesignStatusPending}},
	}
	for _, q := range queries {
		if err := s.db.QueryRowContext(ctx, q.sql, q.args...).Scan(q.dst); err != nil {
			return nil, err
		}
	}
	if err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(size), 0) FROM `blob`").Scan(&o.StorageBytes); err != nil {
		return nil, err
	}
	return &o, nil
}
