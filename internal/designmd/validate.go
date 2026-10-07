// Package designmd implements the DESIGN.md weak-validation pipeline
// (design §10): warnings never block publishing; errors are reserved for
// non-UTF-8 content, empty files and oversized uploads.
package designmd

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Content limits (§10.1).
const MaxContentBytes = 1 << 20 // 1MB

// Check levels shared with the skill validation report.
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

// Report is the response of the designs/validate endpoint: checks only.
type Report struct {
	Checks []Check `json:"checks"`
}

// HasErrors reports whether the content is unpublishable.
func (r *Report) HasErrors() bool {
	for _, c := range r.Checks {
		if c.Level == LevelError {
			return true
		}
	}
	return false
}

func (r *Report) add(level, title, detail string) {
	r.Checks = append(r.Checks, Check{Level: level, Title: title, Detail: detail})
}

// commonSections from §10.2; missing ones produce warnings.
var commonSections = []string{
	"Overview", "Colors", "Typography", "Spacing", "Components", "Elevation", "Guidelines",
}

var hexColorRe = regexp.MustCompile(`#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3})?(?:[0-9a-fA-F]{2})?\b`)

// Validate runs the §10 weak validation over raw DESIGN.md content.
func Validate(content []byte) *Report {
	r := &Report{}
	if len(content) == 0 {
		r.add(LevelError, "空文件", "请上传非空的 DESIGN.md 文本")
		return r
	}
	if len(content) > MaxContentBytes {
		r.add(LevelError, "文件过大", fmt.Sprintf("共 %d 字节,超过 1MB 上限", len(content)))
		return r
	}
	if !utf8.Valid(content) {
		r.add(LevelError, "非 UTF-8 编码", "DESIGN.md 必须是合法的 UTF-8 文本")
		return r
	}
	r.add(LevelOK, "Markdown 可解析", fmt.Sprintf("共 %d 字节", len(content)))

	text := string(content)
	found := map[string]string{} // section -> heading line
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		for _, sec := range commonSections {
			if _, ok := found[sec]; !ok && strings.Contains(strings.ToLower(heading), strings.ToLower(sec)) {
				found[sec] = heading
			}
		}
	}

	var missing []string
	for _, sec := range commonSections {
		if _, ok := found[sec]; !ok {
			missing = append(missing, sec)
		}
	}
	if len(missing) > 0 {
		r.add(LevelWarn, "缺少常见章节",
			"建议补充: "+strings.Join(missing, ", "))
	} else {
		r.add(LevelOK, "章节齐全", "已覆盖 7 个常见章节")
	}

	// hex color counting within the Colors section (§10.2)
	if colors := sectionBody(text, "colors"); colors != "" {
		found := hexColorRe.FindAllString(colors, -1)
		r.add(LevelOK, "颜色定义", fmt.Sprintf("识别出 %d 个颜色定义", len(found)))
	}
	return r
}

// sectionBody returns the text between the heading containing marker
// (case-insensitive) and the next heading of the same or higher level.
func sectionBody(text, marker string) string {
	lines := strings.Split(text, "\n")
	start := -1
	startLevel := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
		heading := strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
		if start < 0 {
			if strings.Contains(heading, marker) {
				start = i + 1
				startLevel = level
			}
			continue
		}
		if level <= startLevel {
			return strings.Join(lines[start:i], "\n")
		}
	}
	if start < 0 {
		return ""
	}
	return strings.Join(lines[start:], "\n")
}
