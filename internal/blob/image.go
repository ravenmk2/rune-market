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

	"golang.org/x/image/draw"

	"github.com/ravenmk2/rune-market/internal/store"
)

// Image rules (§11, §13).
const (
	MaxImageUploadBytes = 5 << 20 // preview images ≤5MB (§10.1)
	MaxImageDimension   = 8192    // decompression-bomb guard (§13)
	PreviewThumbWidth   = 640     // the only derived preview size (§11)
)

// PutImage normalizes an uploaded PNG/JPG: magic sniff → DecodeConfig
// precheck → decode → re-encode PNG → content-addressed store with a _640
// thumbnail (§11). The sha256 is computed over the normalized PNG bytes.
// DB writes go through q (may be a caller transaction).
func (s *Storage) PutImage(ctx context.Context, q store.DBTX, r io.Reader) (sum string, size int64, err error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxImageUploadBytes+1))
	if err != nil {
		return "", 0, err
	}
	if len(raw) > MaxImageUploadBytes {
		return "", 0, fmt.Errorf("image exceeds %dMB limit", MaxImageUploadBytes>>20)
	}
	src, err := decodeUpload(raw)
	if err != nil {
		return "", 0, err
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		return "", 0, fmt.Errorf("cannot re-encode PNG: %w", err)
	}
	normalized := pngBuf.Bytes()
	h := sha256.Sum256(normalized)
	sum = hex.EncodeToString(h[:])
	size = int64(len(normalized))

	// dedup: existing image only bumps the ref count (§11)
	var refs int
	err = q.QueryRowContext(ctx,
		"SELECT ref_count FROM `blob` WHERE sha256 = ?", sum).Scan(&refs)
	switch {
	case err == nil:
		_, err = q.ExecContext(ctx,
			"UPDATE `blob` SET ref_count = ref_count + 1 WHERE sha256 = ?", sum)
		return sum, size, err
	case errors.Is(err, sql.ErrNoRows):
		// new image below
	default:
		return "", 0, err
	}

	dir, err := s.dirFor(KindImage)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	dst, err := s.Path(KindImage, sum)
	if err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(dst, normalized, 0o644); err != nil {
		return "", 0, err
	}
	if err := s.writePreviewThumb(src, dir, sum); err != nil {
		_ = os.Remove(dst)
		return "", 0, err
	}
	if _, err := q.ExecContext(ctx,
		"INSERT INTO `blob` (sha256, kind, size, ref_count, created_at) VALUES (?, ?, ?, 1, ?)",
		sum, KindImage, size, store.Now()); err != nil {
		return "", 0, err
	}
	return sum, size, nil
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
// dimension guard) and decodes the image.
func decodeUpload(raw []byte) (image.Image, error) {
	if !isPNG(raw) && !isJPEG(raw) {
		return nil, errors.New("image must be PNG or JPEG")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("cannot decode image config: %w", err)
	}
	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return nil, fmt.Errorf("image dimensions exceed %dx%d", MaxImageDimension, MaxImageDimension)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s image: %w", format, err)
	}
	return img, nil
}

func isPNG(b []byte) bool {
	return len(b) >= 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G'
}

func isJPEG(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
}
