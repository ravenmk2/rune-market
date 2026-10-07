package skillpkg

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Check levels used by the validation report (contract).
const (
	LevelOK    = "ok"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Check is one line of the validation report (contract shape).
type Check struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// Metadata is the normalized metadata block of the report (contract shape).
type Metadata struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	License       string       `json:"license"`
	Compatibility string       `json:"compatibility"`
	Author        string       `json:"author"`
	Harnesses     []string     `json:"harnesses"`
	Permissions   []Permission `json:"permissions"`
	FileCount     int          `json:"file_count"`
	SHA256        string       `json:"sha256"`
	Size          int64        `json:"size"`
}

// Report is the response of the validate endpoint (contract shape).
type Report struct {
	Checks   []Check   `json:"checks"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

// HasErrors reports whether the package is unpublishable.
func (r *Report) HasErrors() bool {
	for _, c := range r.Checks {
		if c.Level == LevelError {
			return true
		}
	}
	return false
}

// Package is the inspected archive: report plus everything hub needs to
// persist a version.
type Package struct {
	Format      string
	Report      *Report
	TopDir      string // located skill directory; "" = archive-root form
	Body        string // SKILL.md content after the frontmatter
	Frontmatter string // raw frontmatter as JSON (DB passthrough, §9.7)
	Entries     []FileEntry
}

func (p *Package) add(level, title, detail string) {
	p.Report.Checks = append(p.Report.Checks, Check{Level: level, Title: title, Detail: detail})
}

// nameRe is the §9 name rule (also forbids consecutive hyphens).
var nameRe = regexp.MustCompile(`^[a-z0-9](-?[a-z0-9])*$`)

// Inspect runs the full §9 pipeline over an archive on disk and always
// returns a report; a Go error means the file itself was unreadable.
func Inspect(filePath string, size int64, sha256Sum string) (*Package, error) {
	pkg := &Package{Report: &Report{}}

	format, entries, closeFn, err := listEntries(filePath)
	if err != nil {
		return nil, fmt.Errorf("skillpkg: %w", err)
	}
	if closeFn != nil {
		defer func() { _ = closeFn() }()
	}
	if format == "" {
		pkg.add(LevelError, "无法识别压缩包格式", "仅支持 zip / tar / tar.gz(按魔数识别)")
		return pkg, nil
	}
	pkg.Format = format
	pkg.add(LevelOK, "识别压缩包", fmt.Sprintf("格式 %s,大小 %d 字节", format, size))

	// --- §9.1 safety limits ---
	var files []FileEntry
	entryByPath := map[string]*archiveEntry{}
	var total int64
	topDirs := map[string]bool{}
	for i := range entries {
		e := &entries[i]
		if unsafeName(e.Path) {
			pkg.add(LevelError, "拒绝非法路径", fmt.Sprintf("条目 %q 含路径穿越或绝对路径", e.Path))
			continue
		}
		if e.isSymlink {
			pkg.add(LevelError, "拒绝符号链接", fmt.Sprintf("条目 %q 是符号链接", e.Path))
			continue
		}
		if e.isDir {
			continue
		}
		if e.Size > MaxFileSize {
			pkg.add(LevelError, "单文件过大", fmt.Sprintf("%q 为 %d 字节,超过 50MB 上限", e.Path, e.Size))
			continue
		}
		total += e.Size
		files = append(files, e.FileEntry)
		entryByPath[e.Path] = e
		if i := strings.IndexByte(e.Path, '/'); i > 0 {
			topDirs[e.Path[:i]] = true
		}
	}
	if len(files) > MaxFiles {
		pkg.add(LevelError, "文件总数超限", fmt.Sprintf("共 %d 个文件,超过 %d 上限", len(files), MaxFiles))
	}
	if total > MaxTotalSize {
		pkg.add(LevelError, "解压总量超限", fmt.Sprintf("共 %d 字节,超过 200MB 上限(zip bomb 防护)", total))
	}

	// --- §9.2 locate SKILL.md ---
	skillMDPath := ""
	if _, ok := entryByPath["SKILL.md"]; ok {
		skillMDPath = "SKILL.md"
	} else if len(topDirs) == 1 {
		for d := range topDirs {
			if _, ok := entryByPath[d+"/SKILL.md"]; ok {
				skillMDPath = d + "/SKILL.md"
				pkg.TopDir = d
			}
		}
	}
	if skillMDPath == "" {
		pkg.add(LevelError, "未找到 SKILL.md",
			"压缩包根须直接含 SKILL.md,或唯一顶层目录含 SKILL.md")
		return pkg, nil
	}
	pkg.add(LevelOK, "定位 SKILL.md", skillMDPath)

	skillEntry := entryByPath[skillMDPath]
	if skillEntry.Size > MaxSkillMDLen {
		pkg.add(LevelError, "SKILL.md 过大", "单文件上限 1MB")
		return pkg, nil
	}
	rc, err := skillEntry.open()
	if err != nil {
		return nil, fmt.Errorf("skillpkg: read SKILL.md: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(rc, MaxSkillMDLen+1))
	_ = rc.Close()
	if err != nil {
		return nil, fmt.Errorf("skillpkg: read SKILL.md: %w", err)
	}

	// --- §9.3 frontmatter ---
	fmRaw, body, ok := splitFrontmatter(string(raw))
	if !ok {
		pkg.add(LevelError, "缺少 frontmatter", "SKILL.md 须以 --- 包裹的 YAML frontmatter 开头")
		return pkg, nil
	}
	pkg.Body = body
	fm := map[string]any{}
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		pkg.add(LevelError, "frontmatter 解析失败", err.Error())
		return pkg, nil
	}
	pkg.Frontmatter = toJSON(fm)

	meta := &Metadata{
		SHA256:    sha256Sum,
		Size:      size,
		FileCount: len(files),
	}
	pkg.Report.Metadata = meta
	pkg.Entries = files

	// spec 6 field rules (hard validation)
	meta.Name = strField(fm, "name")
	if meta.Name == "" {
		pkg.add(LevelError, "缺少必填字段 name", "frontmatter 必须含 name")
	} else if utf8.RuneCountInString(meta.Name) > 64 || !nameRe.MatchString(meta.Name) {
		pkg.add(LevelError, "name 不合法", "须匹配 ^[a-z0-9](-?[a-z0-9])*$ 且不超过 64 字符")
	}
	meta.Description = strField(fm, "description")
	if meta.Description == "" {
		pkg.add(LevelError, "缺少必填字段 description", "frontmatter 必须含 description")
	} else if utf8.RuneCountInString(meta.Description) > 1024 {
		pkg.add(LevelError, "description 过长", "不超过 1024 字符")
	}
	meta.Compatibility = strField(fm, "compatibility")
	if utf8.RuneCountInString(meta.Compatibility) > 500 {
		pkg.add(LevelError, "compatibility 过长", "不超过 500 字符")
	}
	meta.License = strField(fm, "license")

	// top-directory form: name must match the directory (§9.3)
	if pkg.TopDir != "" && meta.Name != "" && meta.Name != pkg.TopDir {
		pkg.add(LevelError, "name 与目录名不一致",
			fmt.Sprintf("顶层目录为 %q,name 为 %q", pkg.TopDir, meta.Name))
	}

	// extension fields: passthrough with a note (§9.3)
	for _, k := range sortedStrings(extensionFields(fm)) {
		pkg.add(LevelOK, "检测到扩展字段", fmt.Sprintf("`%s` 将原样保留", k))
	}

	// metadata map: author + version hint (§9.5)
	if m, ok := fm["metadata"].(map[string]any); ok {
		meta.Author = strField(m, "author")
		if v := strField(m, "version"); v != "" {
			pkg.add(LevelOK, "检测到 metadata.version",
				fmt.Sprintf("值为 %q;发布版本以表单输入为准", v))
		}
	}

	// --- §9.6 permissions ---
	if raw := strField(fm, "allowed-tools"); raw != "" {
		meta.Permissions = ParseAllowedTools(raw)
	} else if list, ok := fm["allowed-tools"].([]any); ok {
		var parts []string
		for _, it := range list {
			if s, ok := it.(string); ok {
				parts = append(parts, s)
			}
		}
		meta.Permissions = ParseAllowedTools(strings.Join(parts, " "))
	}
	if meta.Permissions == nil {
		meta.Permissions = []Permission{}
	}

	// --- §9.7 harness detection ---
	meta.Harnesses = detectHarnesses(fm, files, pkg.TopDir)
	if meta.Harnesses == nil {
		meta.Harnesses = []string{} // contract: empty array means generic
	}
	if len(meta.Harnesses) == 0 {
		pkg.add(LevelOK, "harness 检测", "仅规范字段,判定为通用 skill")
	} else {
		pkg.add(LevelOK, "harness 检测", "适配: "+strings.Join(meta.Harnesses, ", "))
	}

	// --- §9.4 structure hints (warn, non-blocking) ---
	pkg.structureHints(body, files)
	return pkg, nil
}

// structureHints emits non-blocking §9.4 suggestions.
func (p *Package) structureHints(body string, files []FileEntry) {
	if lines := strings.Count(body, "\n") + 1; lines > 500 {
		p.add(LevelWarn, "正文过长",
			fmt.Sprintf("SKILL.md 正文 %d 行,超过 500 行建议拆分到 references/", lines))
	}
	hasDir := func(prefix string) bool {
		full := p.TopDir + prefix
		for _, f := range files {
			if strings.HasPrefix(f.Path, full) {
				return true
			}
		}
		return false
	}
	bashBlocks := strings.Count(body, "```bash") + strings.Count(body, "```sh")
	if bashBlocks >= 3 && !hasDir("scripts/") {
		p.add(LevelWarn, "建议抽取脚本",
			fmt.Sprintf("正文含 %d 个 shell 代码块,建议抽取为 scripts/ 下的脚本文件", bashBlocks))
	}
	if strings.Count(body, "\n") > 200 && !hasDir("references/") {
		p.add(LevelWarn, "建议补充 references/",
			"正文较长,建议将参考材料拆分到 references/")
	}
}

// splitFrontmatter extracts the --- delimited YAML block at the top.
func splitFrontmatter(content string) (fm, body string, ok bool) {
	// strip a UTF-8 BOM if present
	content = strings.TrimPrefix(content, "\xef\xbb\xbf")
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return "", "", false
}

func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
