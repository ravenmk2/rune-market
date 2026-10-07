package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxTagNameLen bounds tag names (VARCHAR(64)).
const MaxTagNameLen = 64

// ValidateTagName enforces non-empty, ≤64 runes.
func ValidateTagName(name string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	return n > 0 && n <= MaxTagNameLen
}

type TagStore struct {
	db      DBTX
	dialect string
}

func NewTagStore(db DBTX, dialect string) *TagStore {
	return &TagStore{db: db, dialect: dialect}
}

// GetOrCreate returns the id of the tag with the given name.
func (s *TagStore) GetOrCreate(ctx context.Context, name string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM tag WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = NewID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO tag (id, name) VALUES (?, ?)`, id, name); err != nil {
		// concurrent creator won the race: reselect
		if IsUniqueViolation(err) {
			if err := s.db.QueryRowContext(ctx,
				`SELECT id FROM tag WHERE name = ?`, name).Scan(&id); err == nil {
				return id, nil
			}
		}
		return "", err
	}
	return id, nil
}

// SetSkillTags replaces the full tag set of a skill.
func (s *TagStore) SetSkillTags(ctx context.Context, skillID string, names []string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM skill_tag WHERE skill_id = ?`, skillID); err != nil {
		return err
	}
	insert := `INSERT OR IGNORE INTO skill_tag (skill_id, tag_id) VALUES (?, ?)`
	if s.dialect == DialectMySQL {
		insert = `INSERT IGNORE INTO skill_tag (skill_id, tag_id) VALUES (?, ?)`
	}
	for _, name := range names {
		tagID, err := s.GetOrCreate(ctx, name)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, insert, skillID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func (s *TagStore) TagsForSkill(ctx context.Context, skillID string) ([]string, error) {
	m, err := s.TagsBySkillIDs(ctx, []string{skillID})
	if err != nil {
		return nil, err
	}
	return m[skillID], nil
}

// TagsBySkillIDs batch-loads tag names keyed by skill id.
func (s *TagStore) TagsBySkillIDs(ctx context.Context, skillIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(skillIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(skillIDs))
	args := make([]any, len(skillIDs))
	for i, id := range skillIDs {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT st.skill_id, t.name FROM skill_tag st JOIN tag t ON t.id = st.tag_id
		 WHERE st.skill_id IN (`+placeholders[:len(placeholders)-1]+`) ORDER BY t.name`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var skillID, name string
		if err := rows.Scan(&skillID, &name); err != nil {
			return nil, err
		}
		out[skillID] = append(out[skillID], name)
	}
	return out, rows.Err()
}
