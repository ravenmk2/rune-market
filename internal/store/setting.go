package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SettingStore struct {
	db     DBTX
	keyCol string // `key` is reserved in MySQL and needs backticks there
}

func NewSettingStore(db DBTX, dialect string) *SettingStore {
	keyCol := "key"
	if dialect == DialectMySQL {
		keyCol = "`key`"
	}
	return &SettingStore{db: db, keyCol: keyCol}
}

func (s *SettingStore) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT value FROM setting WHERE %s = ?`, s.keyCol), key).
		Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *SettingStore) Set(ctx context.Context, key, value string) error {
	var query string
	if s.keyCol == "`key`" {
		query = `INSERT INTO setting (` + s.keyCol + `, value) VALUES (?, ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value)`
	} else {
		query = `INSERT INTO setting (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	}
	_, err := s.db.ExecContext(ctx, query, key, value)
	return err
}

func (s *SettingStore) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s, value FROM setting`, s.keyCol))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
