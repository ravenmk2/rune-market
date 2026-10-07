package skillpkg

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// archiveBuilder builds test archives in the three supported formats.
type archiveBuilder struct {
	files map[string]string
	// raw entries for hostile cases (traversal, symlink); tar only
	rawEntries []rawEntry
}

type rawEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
}

func newArchive() *archiveBuilder {
	return &archiveBuilder{files: map[string]string{}}
}

const sampleFrontmatter = `---
name: pdf-processing
description: Extract and process PDF documents
license: MIT
compatibility: Requires poppler
allowed-tools: Bash(python3:*) Read Write
metadata:
  author: Raven
  version: 9.9.9
when_to_use: when PDFs show up
---

# PDF Processing

Body text here.
`

func (b *archiveBuilder) buildZip(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "skill.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	for name, body := range b.files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func (b *archiveBuilder) buildTar(t *testing.T, gz bool) string {
	t.Helper()
	name := "skill.tar"
	if gz {
		name += ".gz"
	}
	p := filepath.Join(t.TempDir(), name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var w io.Writer = f
	var gzw *gzip.Writer
	if gz {
		gzw = gzip.NewWriter(f)
		w = gzw
	}
	tw := tar.NewWriter(w)
	write := func(h *tar.Header, body string) {
		t.Helper()
		h.Size = int64(len(body))
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := io.WriteString(tw, body); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, body := range b.files {
		write(&tar.Header{Name: name, Mode: 0o644, Typeflag: tar.TypeReg}, body)
	}
	for _, re := range b.rawEntries {
		write(&tar.Header{
			Name: re.name, Mode: 0o644, Typeflag: re.typeflag, Linkname: re.linkname,
		}, re.body)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if gzw != nil {
		if err := gzw.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func fileSize(t *testing.T, p string) int64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}

func inspect(t *testing.T, path string) *Package {
	t.Helper()
	pkg, err := Inspect(path, fileSize(t, path), "deadbeef")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	return pkg
}

func errorChecks(pkg *Package) []Check {
	var out []Check
	for _, c := range pkg.Report.Checks {
		if c.Level == LevelError {
			out = append(out, c)
		}
	}
	return out
}

func warnChecks(pkg *Package) []Check {
	var out []Check
	for _, c := range pkg.Report.Checks {
		if c.Level == LevelWarn {
			out = append(out, c)
		}
	}
	return out
}

func TestInspectZipTopDirForm(t *testing.T) {
	b := newArchive()
	b.files["pdf-processing/SKILL.md"] = sampleFrontmatter
	b.files["pdf-processing/scripts/extract.py"] = "print('hi')"
	b.files["pdf-processing/agents/openai.yaml"] = "name: pdf"
	p := b.buildZip(t)

	pkg := inspect(t, p)
	if errs := errorChecks(pkg); len(errs) > 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if pkg.Format != FormatZip || pkg.TopDir != "pdf-processing" {
		t.Fatalf("format=%q topdir=%q", pkg.Format, pkg.TopDir)
	}

	m := pkg.Report.Metadata
	if m == nil {
		t.Fatal("metadata missing")
	}
	if m.Name != "pdf-processing" || m.Description == "" || m.License != "MIT" ||
		m.Compatibility != "Requires poppler" || m.Author != "Raven" {
		t.Fatalf("metadata: %+v", m)
	}
	if m.FileCount != 3 || m.SHA256 != "deadbeef" || m.Size != fileSize(t, p) {
		t.Fatalf("counts: %+v", m)
	}

	// harnesses: when_to_use → claude-code, agents/openai.yaml → codex
	want := []string{HarnessClaudeCode, HarnessCodex}
	if strings.Join(m.Harnesses, ",") != strings.Join(want, ",") {
		t.Fatalf("harnesses: %v", m.Harnesses)
	}

	// permissions: paren-aware split + risk annotation
	if len(m.Permissions) != 3 {
		t.Fatalf("permissions: %+v", m.Permissions)
	}
	if m.Permissions[0].Tool != "Bash(python3:*)" || m.Permissions[0].Risk != "warn" {
		t.Fatalf("bash permission: %+v", m.Permissions[0])
	}
	if m.Permissions[1].Tool != "Read" || m.Permissions[1].Risk != "ok" {
		t.Fatalf("read permission: %+v", m.Permissions[1])
	}

	// extension fields and version hint are informational
	var extFound, verFound bool
	for _, c := range pkg.Report.Checks {
		if c.Level == LevelOK && strings.Contains(c.Detail, "when_to_use") {
			extFound = true
		}
		if c.Level == LevelOK && strings.Contains(c.Detail, "9.9.9") {
			verFound = true
		}
	}
	if !extFound || !verFound {
		t.Fatalf("expected extension-field and version hints: %+v", pkg.Report.Checks)
	}

	// frontmatter passthrough keeps extension fields
	if !strings.Contains(pkg.Frontmatter, "when_to_use") {
		t.Fatalf("frontmatter passthrough: %s", pkg.Frontmatter)
	}
}

func TestInspectTarRootFormGeneric(t *testing.T) {
	fm := "---\nname: plain\ndescription: A plain skill\n---\n\nBody\n"
	b := newArchive()
	b.files["SKILL.md"] = fm
	p := b.buildTar(t, false)

	pkg := inspect(t, p)
	if errs := errorChecks(pkg); len(errs) > 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if pkg.Format != FormatTar || pkg.TopDir != "" {
		t.Fatalf("format=%q topdir=%q", pkg.Format, pkg.TopDir)
	}
	if len(pkg.Report.Metadata.Harnesses) != 0 {
		t.Fatalf("expected generic harnesses, got %v", pkg.Report.Metadata.Harnesses)
	}
	// contract: generic marshals as [], never null
	raw, err := json.Marshal(pkg.Report.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"harnesses":[]`) {
		t.Fatalf("harnesses should be []: %s", raw)
	}
}

func TestInspectTarGz(t *testing.T) {
	b := newArchive()
	b.files["pack/SKILL.md"] = "---\nname: pack\ndescription: packed\n---\n\nBody\n"
	p := b.buildTar(t, true)

	pkg := inspect(t, p)
	if errs := errorChecks(pkg); len(errs) > 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	if pkg.Format != FormatTarGz {
		t.Fatalf("format=%q", pkg.Format)
	}
}

func TestInspectHostilePath(t *testing.T) {
	b := newArchive()
	b.files["ok/SKILL.md"] = "---\nname: ok\ndescription: d\n---\n\nBody\n"
	b.rawEntries = append(b.rawEntries, rawEntry{
		name: "../evil.sh", body: "rm -rf /", typeflag: tar.TypeReg,
	})
	p := b.buildTar(t, false)

	pkg := inspect(t, p)
	var found bool
	for _, c := range errorChecks(pkg) {
		if strings.Contains(c.Title, "非法路径") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected traversal error: %+v", pkg.Report.Checks)
	}
}

func TestInspectSymlink(t *testing.T) {
	b := newArchive()
	b.files["ok/SKILL.md"] = "---\nname: ok\ndescription: d\n---\n\nBody\n"
	b.rawEntries = append(b.rawEntries, rawEntry{
		name: "ok/link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd",
	})
	p := b.buildTar(t, false)

	pkg := inspect(t, p)
	var found bool
	for _, c := range errorChecks(pkg) {
		if strings.Contains(c.Title, "符号链接") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected symlink error: %+v", pkg.Report.Checks)
	}
}

func TestInspectTooManyFiles(t *testing.T) {
	b := newArchive()
	b.files["big/SKILL.md"] = "---\nname: big\ndescription: d\n---\n\nBody\n"
	for i := 0; i < MaxFiles; i++ {
		b.files["big/f"+strconv.Itoa(i)] = "x"
	}
	p := b.buildTar(t, false)

	pkg := inspect(t, p)
	var found bool
	for _, c := range errorChecks(pkg) {
		if strings.Contains(c.Title, "文件总数超限") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected file count error: %+v", pkg.Report.Checks)
	}
}

func TestInspectMissingSkillMD(t *testing.T) {
	b := newArchive()
	b.files["README.md"] = "nothing here"
	p := b.buildZip(t)

	pkg := inspect(t, p)
	var found bool
	for _, c := range errorChecks(pkg) {
		if strings.Contains(c.Title, "未找到 SKILL.md") {
			found = true
		}
	}
	if !found || pkg.Report.Metadata != nil {
		t.Fatalf("expected SKILL.md error and no metadata: %+v", pkg.Report)
	}
}

func TestInspectNameDirMismatch(t *testing.T) {
	b := newArchive()
	b.files["other-name/SKILL.md"] = "---\nname: mine\ndescription: d\n---\n\nBody\n"
	p := b.buildZip(t)

	pkg := inspect(t, p)
	var found bool
	for _, c := range errorChecks(pkg) {
		if strings.Contains(c.Title, "目录名不一致") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected name/dir mismatch error: %+v", pkg.Report.Checks)
	}
}

func TestInspectMissingDescription(t *testing.T) {
	b := newArchive()
	b.files["SKILL.md"] = "---\nname: x\ndescription: \"\"\n---\n\nBody\n"
	p := b.buildZip(t)

	pkg := inspect(t, p)
	if !pkg.Report.HasErrors() {
		t.Fatalf("expected error for empty description: %+v", pkg.Report.Checks)
	}
}

func TestInspectLongBodyWarn(t *testing.T) {
	body := strings.Repeat("line\n", 600)
	b := newArchive()
	b.files["SKILL.md"] = "---\nname: long\ndescription: d\n---\n\n" + body
	p := b.buildZip(t)

	pkg := inspect(t, p)
	if pkg.Report.HasErrors() {
		t.Fatalf("should not error: %+v", errorChecks(pkg))
	}
	if len(warnChecks(pkg)) == 0 {
		t.Fatalf("expected structure warnings: %+v", pkg.Report.Checks)
	}
}

func TestInspectUnknownFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "junk.bin")
	if err := os.WriteFile(p, []byte("not an archive at all.........."), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := inspect(t, p)
	if !pkg.Report.HasErrors() {
		t.Fatalf("expected format error: %+v", pkg.Report.Checks)
	}
}

func TestTreeAndReadFile(t *testing.T) {
	b := newArchive()
	b.files["pdf-processing/SKILL.md"] = sampleFrontmatter
	b.files["pdf-processing/data.bin"] = "\x00\x01\x02"
	zipPath := b.buildZip(t)
	tarPath := b.buildTarFromFiles(t)

	for _, p := range []string{zipPath, tarPath} {
		tree, err := Tree(p)
		if err != nil {
			t.Fatalf("tree %s: %v", p, err)
		}
		if len(tree) != 2 {
			t.Fatalf("tree %s: %+v", p, tree)
		}

		content, size, err := ReadFile(p, "pdf-processing/SKILL.md", 1<<20)
		if err != nil || !strings.Contains(string(content), "pdf-processing") || size <= 0 {
			t.Fatalf("readfile %s: %v size=%d", p, err, size)
		}

		// over-limit read returns size without content
		content, size, err = ReadFile(p, "pdf-processing/SKILL.md", 10)
		if err != nil || content != nil || size <= 10 {
			t.Fatalf("limit read: %v content=%d size=%d", err, len(content), size)
		}

		if _, _, err := ReadFile(p, "../escape", 1<<20); err == nil {
			t.Fatal("expected traversal rejection")
		}
		if _, _, err := ReadFile(p, "nope.txt", 1<<20); err == nil {
			t.Fatal("expected not-exist error")
		}
	}
}

func (b *archiveBuilder) buildTarFromFiles(t *testing.T) string {
	t.Helper()
	return b.buildTar(t, false)
}

func TestIsTextPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"a/SKILL.md", true},
		{"a/b.yaml", true},
		{"a/script.py", true},
		{"a/data.bin", false},
		{"a/logo.png", false},
	} {
		if got := IsTextPath(tc.path); got != tc.want {
			t.Fatalf("IsTextPath(%q)=%v want %v", tc.path, got, tc.want)
		}
	}
}

func TestSplitTools(t *testing.T) {
	got := splitTools("Bash(npm run test:*) Read Write Edit Bash(git status)")
	want := []string{"Bash(npm run test:*)", "Read", "Write", "Edit", "Bash(git status)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := splitTools(""); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
}

func TestParseAllowedToolsList(t *testing.T) {
	// allowed-tools as a YAML list must also parse
	fm := "---\nname: l\ndescription: d\nallowed-tools:\n  - Bash(ls:*)\n  - Read\n---\n\nBody\n"
	b := newArchive()
	b.files["SKILL.md"] = fm
	pkg := inspect(t, b.buildZip(t))
	if errs := errorChecks(pkg); len(errs) > 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	perms := pkg.Report.Metadata.Permissions
	if len(perms) != 2 || perms[0].Tool != "Bash(ls:*)" || perms[0].Risk != "warn" {
		t.Fatalf("perms: %+v", perms)
	}
}

func TestDetectFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	gzw := gzip.NewWriter(buf)
	_, _ = gzw.Write([]byte("x"))
	_ = gzw.Close()

	b := newArchive()
	b.files["SKILL.md"] = "---\nname: f\ndescription: d\n---\n\nB\n"
	for _, tc := range []struct {
		path, want string
	}{
		{b.buildZip(t), FormatZip},
		{b.buildTar(t, false), FormatTar},
		{b.buildTar(t, true), FormatTarGz},
	} {
		got, err := DetectFormat(tc.path)
		if err != nil || got != tc.want {
			t.Fatalf("DetectFormat(%s)=%q,%v want %q", tc.path, got, err, tc.want)
		}
	}
}
