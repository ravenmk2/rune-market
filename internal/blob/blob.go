// Package blob is the content-addressed store for immutable content
// (design §11): files live at <kind dir>/<sha256>, accounted in the blob
// table with ref counting; zero refs deletes the file.
package blob

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ravenmk2/rune-market/internal/store"
)

// Kinds select the storage directory (design §4).
const (
	KindArchive = "archive" // data/blobs/<sha256>      (no extension)
	KindImage   = "image"   // data/images/<sha256>.png (M3)
)

var errUnknownKind = errors.New("blob: unknown kind")

type Storage struct {
	dataDir string
}

func New(dataDir string) *Storage {
	return &Storage{dataDir: dataDir}
}

func (s *Storage) dirFor(kind string) (string, error) {
	switch kind {
	case KindArchive:
		return filepath.Join(s.dataDir, "blobs"), nil
	case KindImage:
		return filepath.Join(s.dataDir, "images"), nil
	default:
		return "", errUnknownKind
	}
}

func fileName(kind, sum string) string {
	if kind == KindImage {
		return sum + ".png"
	}
	return sum
}

// Path returns the on-disk path for a blob.
func (s *Storage) Path(kind, sum string) (string, error) {
	dir, err := s.dirFor(kind)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName(kind, sum)), nil
}

// Put streams r into the store. Existing content short-circuits with
// ref_count+1 (instant re-upload, §11). DB writes go through q, which may
// be a caller transaction; the file is moved into place immediately.
func (s *Storage) Put(ctx context.Context, q store.DBTX, kind string, r io.Reader) (sum string, size int64, err error) {
	dir, err := s.dirFor(kind)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("blob: create dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", 0, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	h := sha256.New()
	size, err = io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		_ = tmp.Close()
		return "", 0, fmt.Errorf("blob: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}
	sum = hex.EncodeToString(h.Sum(nil))

	var refs int
	err = q.QueryRowContext(ctx,
		"SELECT ref_count FROM `blob` WHERE sha256 = ?", sum).Scan(&refs)
	switch {
	case err == nil:
		_, err = q.ExecContext(ctx,
			"UPDATE `blob` SET ref_count = ref_count + 1 WHERE sha256 = ?", sum)
		return sum, size, err
	case errors.Is(err, sql.ErrNoRows):
		// new content below
	default:
		return "", 0, err
	}

	if _, err := q.ExecContext(ctx,
		"INSERT INTO `blob` (sha256, kind, size, ref_count, created_at) VALUES (?, ?, ?, 1, ?)",
		sum, kind, size, store.Now()); err != nil {
		return "", 0, err
	}
	dst, err := s.Path(kind, sum)
	if err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return "", 0, fmt.Errorf("blob: move into place: %w", err)
	}
	return sum, size, nil
}

// Release decrements the ref count; at zero it deletes the row, the file
// and any derived thumbnails (<sha256>_*.png, §11).
func (s *Storage) Release(ctx context.Context, q store.DBTX, sum string) error {
	if _, err := q.ExecContext(ctx,
		"UPDATE `blob` SET ref_count = ref_count - 1 WHERE sha256 = ?", sum); err != nil {
		return err
	}
	var kind string
	var refs int
	err := q.QueryRowContext(ctx,
		"SELECT kind, ref_count FROM `blob` WHERE sha256 = ?", sum).Scan(&kind, &refs)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.maybeDelete(ctx, q, kind, sum, refs)
}

func (s *Storage) maybeDelete(ctx context.Context, q store.DBTX, kind, sum string, refs int) error {
	if refs > 0 {
		return nil
	}
	if _, err := q.ExecContext(ctx, "DELETE FROM `blob` WHERE sha256 = ?", sum); err != nil {
		return err
	}
	dir, err := s.dirFor(kind)
	if err != nil {
		return err
	}
	base := filepath.Join(dir, sum)
	_ = os.Remove(base)
	_ = os.Remove(base + ".png")
	// derived thumbnails share the <sha256>_ prefix (§11)
	matches, _ := filepath.Glob(base + "_*.png")
	for _, m := range matches {
		_ = os.Remove(m)
	}
	return nil
}

// Open opens a blob file for streaming reads.
func (s *Storage) Open(kind, sum string) (*os.File, error) {
	if strings.Contains(sum, "/") || strings.Contains(sum, `\`) {
		return nil, fmt.Errorf("blob: bad sha256 %q", sum)
	}
	p, err := s.Path(kind, sum)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}
