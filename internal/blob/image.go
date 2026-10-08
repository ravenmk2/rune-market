package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // JPEG decode
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	"github.com/ravenmk2/rune-market/internal/store"
)

// Image rules (§11, §13).
const (
	MaxImageUploadBytes = 5 << 20 // preview images ≤5MB (§10.1)
	MaxImageDimension   = 8192    // decompression-bomb guard (§13)
	PreviewThumbWidth   = 640     // the only derived preview size (§11)
)

// PutImage stores an uploaded PNG/JPG in its original format: magic sniff →
// DecodeConfig precheck → the original bytes land at <sha>.<ext> with the
// sha256 computed over them; a _640 PNG thumbnail is derived for list cards
// (§11). The blob row records the ext for URL building.
// DB writes go through q (may be a caller transaction).
func (s *Storage) PutImage(ctx context.Context, q store.DBTX, r io.Reader) (sum, ext string, size int64, err error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxImageUploadBytes+1))
	if err != nil {
		return "", "", 0, err
	}
	if len(raw) > MaxImageUploadBytes {
		return "", "", 0, fmt.Errorf("image exceeds %dMB limit", MaxImageUploadBytes>>20)
	}
	src, ext, err := decodeUpload(raw)
	if err != nil {
		return "", "", 0, err
	}

	h := sha256.Sum256(raw)
	sum = hex.EncodeToString(h[:])
	size = int64(len(raw))

	// dedup: existing image only bumps the ref count (§11)
	var refs int
	var existingKind string
	err = q.QueryRowContext(ctx,
		"SELECT kind, ref_count FROM `blob` WHERE sha256 = ?", sum).Scan(&existingKind, &refs)
	switch {
	case err == nil:
		if existingKind != KindImage {
			return "", "", 0, fmt.Errorf("blob %s already exists with kind %q", sum, existingKind)
		}
		_, err = q.ExecContext(ctx,
			"UPDATE `blob` SET ref_count = ref_count + 1 WHERE sha256 = ?", sum)
		return sum, ext, size, err
	case errors.Is(err, sql.ErrNoRows):
		// new image below
	default:
		return "", "", 0, err
	}

	dir, err := s.dirFor(KindImage)
	if err != nil {
		return "", "", 0, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", 0, err
	}
	dst, err := s.Path(KindImage, sum, ext)
	if err != nil {
		return "", "", 0, err
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return "", "", 0, err
	}
	if err := s.writePreviewThumb(src, dir, sum); err != nil {
		_ = os.Remove(dst)
		return "", "", 0, err
	}
	if _, err := q.ExecContext(ctx,
		"INSERT INTO `blob` (sha256, kind, size, ext, ref_count, created_at) VALUES (?, ?, ?, ?, 1, ?)",
		sum, KindImage, size, ext, store.Now()); err != nil {
		return "", "", 0, err
	}
	return sum, ext, size, nil
}

// ImageExt returns the stored extension ("png"/"jpg") of an image blob,
// or store.ErrNotFound.
func (s *Storage) ImageExt(ctx context.Context, q store.DBTX, sum string) (string, error) {
	var ext string
	err := q.QueryRowContext(ctx,
		"SELECT ext FROM `blob` WHERE sha256 = ? AND kind = ?", sum, KindImage).Scan(&ext)
	if errors.Is(err, sql.ErrNoRows) {
		return "", store.ErrNotFound
	}
	return ext, err
}

// ImageExts batch-loads image extensions for URL building (avoids N+1 on
// list endpoints); unknown or non-image shas are simply absent from the map.
func (s *Storage) ImageExts(ctx context.Context, q store.DBTX, sums []string) (map[string]string, error) {
	out := map[string]string{}
	seen := map[string]bool{}
	var uniq []string
	for _, sum := range sums {
		if sum != "" && !seen[sum] {
			seen[sum] = true
			uniq = append(uniq, sum)
		}
	}
	if len(uniq) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(uniq))
	args := make([]any, 0, len(uniq)+1)
	args = append(args, KindImage)
	for _, sum := range uniq {
		args = append(args, sum)
	}
	rows, err := q.QueryContext(ctx,
		"SELECT sha256, ext FROM `blob` WHERE kind = ? AND sha256 IN ("+
			placeholders[:len(placeholders)-1]+")", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var sum, ext string
		if err := rows.Scan(&sum, &ext); err != nil {
			return nil, err
		}
		out[sum] = ext
	}
	return out, rows.Err()
}

// writePreviewThumb writes the _640 derived thumbnail (width 640, aspect
// preserved; smaller originals are kept as-is, never upscaled).
func (s *Storage) writePreviewThumb(src image.Image, dir, sum string) error {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := src
	if w > PreviewThumbWidth {
		nh := int(float64(h) * float64(PreviewThumbWidth) / float64(w))
		scaled := image.NewRGBA(image.Rect(0, 0, PreviewThumbWidth, nh))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, b, draw.Over, nil)
		dst = scaled
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, sum+"_640.png"), buf.Bytes(), 0o644)
}

// AddRef bumps the ref count of an existing blob, verifying its kind.
// Used when a version references a previously uploaded preview (§10.3).
func (s *Storage) AddRef(ctx context.Context, q store.DBTX, sum, expectKind string) error {
	var kind string
	err := q.QueryRowContext(ctx,
		"SELECT kind FROM `blob` WHERE sha256 = ?", sum).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}
	if kind != expectKind {
		return fmt.Errorf("blob %s has kind %q, expected %q", sum, kind, expectKind)
	}
	_, err = q.ExecContext(ctx,
		"UPDATE `blob` SET ref_count = ref_count + 1 WHERE sha256 = ?", sum)
	return err
}

// decodeUpload runs the §13 preflight checks (magic sniff, DecodeConfig
// dimension guard), decodes the image for thumbnail derivation, and reports
// the original format extension ("png"/"jpg").
func decodeUpload(raw []byte) (image.Image, string, error) {
	var ext string
	switch {
	case isPNG(raw):
		ext = "png"
	case isJPEG(raw):
		ext = "jpg"
	default:
		return nil, "", errors.New("image must be PNG or JPEG")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("cannot decode image config: %w", err)
	}
	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return nil, "", fmt.Errorf("image dimensions exceed %dx%d", MaxImageDimension, MaxImageDimension)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("cannot decode %s image: %w", format, err)
	}
	return img, ext, nil
}

func isPNG(b []byte) bool {
	return len(b) >= 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G'
}

func isJPEG(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
}
