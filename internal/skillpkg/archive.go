// Package skillpkg parses skill archives: format detection, safety limits,
// SKILL.md location, frontmatter validation, permission parsing and harness
// detection (design §9). Archives are always streamed, never extracted to
// disk (§9 文件预览).
package skillpkg

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// Archive formats recognized by magic bytes.
const (
	FormatZip   = "zip"
	FormatTar   = "tar"
	FormatTarGz = "tar.gz"
)

// Safety limits from §9.1.
const (
	MaxFileSize   = 50 << 20  // per file
	MaxFiles      = 2000      // entry count
	MaxTotalSize  = 200 << 20 // uncompressed total (zip bomb guard)
	MaxSkillMDLen = 1 << 20   // SKILL.md read cap
)

// FileEntry is one regular file in the archive (contract: flat list).
type FileEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type archiveEntry struct {
	FileEntry
	isDir     bool
	isSymlink bool
	open      func() (io.ReadCloser, error) // nil for directories
}

// DetectFormat sniffs the archive format by magic bytes.
func DetectFormat(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 262)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	buf = buf[:n]
	switch {
	case n >= 4 && string(buf[:2]) == "PK":
		return FormatZip, nil
	case n >= 2 && buf[0] == 0x1f && buf[1] == 0x8b:
		return FormatTarGz, nil
	case n >= 262 && string(buf[257:262]) == "ustar":
		return FormatTar, nil
	default:
		return "", nil
	}
}

// unsafeName reports path traversal / absolute path / drive letter attempts.
func unsafeName(name string) bool {
	if name == "" || strings.ContainsRune(name, 0) {
		return true
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) || path.IsAbs(name) {
		return true
	}
	if len(name) >= 2 && name[1] == ':' {
		return true
	}
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return true
		}
	}
	return false
}

// listEntries streams the archive once and returns all entries.
// open funcs remain usable until the returned closer is called.
func listEntries(filePath string) (format string, entries []archiveEntry, closeFn func() error, err error) {
	format, err = DetectFormat(filePath)
	if err != nil {
		return "", nil, nil, err
	}
	switch format {
	case FormatZip:
		return listZip(filePath)
	case FormatTar, FormatTarGz:
		return listTar(filePath, format == FormatTarGz)
	default:
		return "", nil, nil, nil
	}
}

func listZip(filePath string) (string, []archiveEntry, func() error, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return FormatZip, nil, nil, fmt.Errorf("open zip: %w", err)
	}
	entries := make([]archiveEntry, 0, len(r.File))
	for _, f := range r.File {
		e := archiveEntry{
			FileEntry: FileEntry{Path: f.Name, Size: int64(f.UncompressedSize64)},
			isDir:     f.FileInfo().IsDir(),
			isSymlink: f.FileInfo().Mode()&os.ModeSymlink != 0,
		}
		if !e.isDir && !e.isSymlink {
			f := f
			e.open = func() (io.ReadCloser, error) { return f.Open() }
		}
		entries = append(entries, e)
	}
	return FormatZip, entries, r.Close, nil
}

func listTar(filePath string, gzipped bool) (string, []archiveEntry, func() error, error) {
	format := FormatTar
	if gzipped {
		format = FormatTarGz
	}
	var entries []archiveEntry
	err := scanTar(filePath, gzipped, func(h *tar.Header, tr *tar.Reader) error {
		e := archiveEntry{
			FileEntry: FileEntry{Path: h.Name, Size: h.Size},
			isDir:     h.Typeflag == tar.TypeDir,
			isSymlink: h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink,
		}
		if !e.isDir && !e.isSymlink {
			name := h.Name
			e.open = func() (io.ReadCloser, error) {
				return openTarEntry(filePath, gzipped, name)
			}
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return format, nil, nil, err
	}
	return format, entries, func() error { return nil }, nil
}

// scanTar streams a tar (optionally gzipped) once, invoking fn per header.
// When fn is invoked the reader is positioned at the entry data.
func scanTar(filePath string, gzipped bool, fn func(h *tar.Header, tr *tar.Reader) error) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var r io.Reader = f
	if gzipped {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("open gzip: %w", err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if err := fn(h, tr); err != nil {
			return err
		}
	}
}

// openTarEntry re-streams the archive to read one entry (tar has no random
// access; this keeps the "no extraction to disk" rule, §9).
func openTarEntry(filePath string, gzipped bool, name string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	var r io.Reader = f
	var gz *gzip.Reader
	if gzipped {
		gz, err = gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		r = gz
	}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			_ = f.Close()
			return nil, os.ErrNotExist
		}
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		if h.Name == name {
			return &tarEntryReader{ReadCloser: f, gz: gz, tr: tr}, nil
		}
	}
}

// tarEntryReader ties the underlying file's lifetime to the entry reader.
type tarEntryReader struct {
	io.ReadCloser // the *os.File
	gz            *gzip.Reader
	tr            *tar.Reader
}

func (t *tarEntryReader) Read(p []byte) (int, error) { return t.tr.Read(p) }

func (t *tarEntryReader) Close() error {
	if t.gz != nil {
		_ = t.gz.Close()
	}
	return t.ReadCloser.Close()
}

// Tree lists all regular files in the archive (flat, archive-relative).
func Tree(filePath string) ([]FileEntry, error) {
	_, entries, closeFn, err := listEntries(filePath)
	if err != nil {
		return nil, err
	}
	if closeFn != nil {
		defer func() { _ = closeFn() }()
	}
	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		if !e.isDir && !e.isSymlink && !unsafeName(e.Path) {
			out = append(out, e.FileEntry)
		}
	}
	return out, nil
}

// ReadFile reads one entry fully when its size is within limit; otherwise
// it returns nil content with the real size (handler decides what to do).
func ReadFile(filePath, name string, limit int64) (content []byte, size int64, err error) {
	if unsafeName(name) {
		return nil, 0, fmt.Errorf("unsafe path %q", name)
	}
	_, entries, closeFn, err := listEntries(filePath)
	if err != nil {
		return nil, 0, err
	}
	if closeFn != nil {
		defer func() { _ = closeFn() }()
	}
	for _, e := range entries {
		if e.Path != name || e.isDir || e.isSymlink {
			continue
		}
		if e.Size > limit {
			return nil, e.Size, nil
		}
		rc, err := e.open()
		if err != nil {
			return nil, 0, err
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(io.LimitReader(rc, limit+1))
		if err != nil {
			return nil, 0, err
		}
		return data, int64(len(data)), nil
	}
	return nil, 0, os.ErrNotExist
}

// textExtensions drives the text/binary decision for file preview (§9).
var textExtensions = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".json": true,
	".yaml": true, ".yml": true, ".toml": true, ".xml": true,
	".py": true, ".js": true, ".mjs": true, ".ts": true, ".tsx": true,
	".jsx": true, ".go": true, ".rs": true, ".sh": true, ".bash": true,
	".zsh": true, ".ps1": true, ".html": true, ".css": true,
	".csv": true, ".sql": true, ".ini": true, ".cfg": true,
	".env.example": true, ".gitignore": true, ".dockerignore": true,
}

// IsTextPath reports whether a path should be previewed as text.
func IsTextPath(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return textExtensions[ext]
}
