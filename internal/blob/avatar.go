package blob

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// Avatar rules (§11): user-addressed (avatars/<user_id>.png), overwrite
// writes, derived squares at the whitelist sizes; never enters the blob
// table — user.has_avatar is the only DB marker.
var AvatarSizes = []int{32, 64, 128}

func (s *Storage) avatarsDir() string {
	return filepath.Join(s.dataDir, "avatars")
}

// AvatarPath returns the on-disk path of an avatar file; size "" means the
// original, otherwise one of "32"/"64"/"128".
func (s *Storage) AvatarPath(userID, size string) string {
	name := userID
	if size != "" {
		name += "_" + size
	}
	return filepath.Join(s.avatarsDir(), name+".png")
}

// PutAvatar normalizes an uploaded PNG/JPG into the avatar file group:
// center-crop to a square, re-encode PNG, overwrite the original, and
// regenerate the 32/64/128 thumbnails (§11).
func (s *Storage) PutAvatar(userID string, r io.Reader) error {
	raw, err := io.ReadAll(io.LimitReader(r, MaxImageUploadBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > MaxImageUploadBytes {
		return fmt.Errorf("image exceeds %dMB limit", MaxImageUploadBytes>>20)
	}
	src, err := decodeUpload(raw)
	if err != nil {
		return err
	}
	square := centerCropSquare(src)

	if err := os.MkdirAll(s.avatarsDir(), 0o755); err != nil {
		return err
	}
	if err := writePNG(s.AvatarPath(userID, ""), square); err != nil {
		return err
	}
	for _, size := range AvatarSizes {
		scaled := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), square, square.Bounds(), draw.Over, nil)
		if err := writePNG(s.AvatarPath(userID, fmt.Sprint(size)), scaled); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAvatar removes the avatar file group of a user.
func (s *Storage) DeleteAvatar(userID string) error {
	matches, err := filepath.Glob(filepath.Join(s.avatarsDir(), userID+"*.png"))
	if err != nil {
		return err
	}
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			return err
		}
	}
	return nil
}

// AvatarBytes returns the total size of all avatar files (§6.3 storage
// accounting: avatars are scanned on demand).
func (s *Storage) AvatarBytes() (int64, error) {
	var total int64
	err := filepath.WalkDir(s.avatarsDir(), func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}

// centerCropSquare crops the centered min(w,h) square out of src.
func centerCropSquare(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	side := min(w, h)
	x0 := b.Min.X + (w-side)/2
	y0 := b.Min.Y + (h-side)/2
	out := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(out, out.Bounds(), src, image.Point{X: x0, Y: y0}, draw.Src)
	return out
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
