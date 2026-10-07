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
	return s.setTags(ctx, "skill_tag", "skill_id", skillID, names)
}

// SetDesignTags replaces the full tag set of a designmd.
func (s *TagStore) SetDesignTags(ctx context.Context, designID string, names []string) error {
	return s.setTags(ctx, "designmd_tag", "designmd_id", designID, names)
}

// setTags replaces the full tag set in the given link table.
func (s *TagStore) setTags(ctx context.Context, linkTable, ownerCol, ownerID string, names []string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM `+linkTable+` WHERE `+ownerCol+` = ?`, ownerID); err != nil {
		return err
	}
	insert := `INSERT OR IGNORE INTO ` + linkTable + ` (` + ownerCol + `, tag_id) VALUES (?, ?)`
	if s.dialect == DialectMySQL {
		insert = `INSERT IGNORE INTO ` + linkTable + ` (` + ownerCol + `, tag_id) VALUES (?, ?)`
	}
	for _, name := range names {
		tagID, err := s.GetOrCreate(ctx, name)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, insert, ownerID, tagID); err != nil {
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

func (s *TagStore) TagsForDesign(ctx context.Context, designID string) ([]string, error) {
	m, err := s.TagsByDesignIDs(ctx, []string{designID})
	if err != nil {
		return nil, err
	}
	return m[designID], nil
}

// TagsBySkillIDs batch-loads tag names keyed by skill id.
func (s *TagStore) TagsBySkillIDs(ctx context.Context, skillIDs []string) (map[string][]string, error) {
	return s.tagsByOwnerIDs(ctx, "skill_tag", "skill_id", skillIDs)
}

// TagsByDesignIDs batch-loads tag names keyed by designmd id.
func (s *TagStore) TagsByDesignIDs(ctx context.Context, designIDs []string) (map[string][]string, error) {
	return s.tagsByOwnerIDs(ctx, "designmd_tag", "designmd_id", designIDs)
}

func (s *TagStore) tagsByOwnerIDs(ctx context.Context, linkTable, ownerCol string, ownerIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(ownerIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ownerIDs))
	args := make([]any, len(ownerIDs))
	for i, id := range ownerIDs {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT lt.`+ownerCol+`, t.name FROM `+linkTable+` lt JOIN tag t ON t.id = lt.tag_id
		 WHERE lt.`+ownerCol+` IN (`+placeholders[:len(placeholders)-1]+`) ORDER BY t.name`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var ownerID, name string
		if err := rows.Scan(&ownerID, &name); err != nil {
			return nil, err
		}
		out[ownerID] = append(out[ownerID], name)
	}
	return out, rows.Err()
}
