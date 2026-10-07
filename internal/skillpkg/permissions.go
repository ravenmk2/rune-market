package skillpkg

import (
	"strings"
)

// Permission is one parsed allowed-tools entry (contract shape).
type Permission struct {
	Tool string `json:"tool"` // full spec, e.g. "Bash(python3:*)"
	Risk string `json:"risk"` // ok | warn
	Note string `json:"note"`
}

// toolRisks maps the tool base name to its risk annotation (§9.6:
// Bash/Write/Edit are warnings, Read-like tools are safe).
var toolRisks = map[string]Permission{
	"Bash":  {Risk: "warn", Note: "执行命令"},
	"Write": {Risk: "warn", Note: "写入文件"},
	"Edit":  {Risk: "warn", Note: "修改文件"},
	"Read":  {Risk: "ok", Note: "读取文件"},
	"Glob":  {Risk: "ok", Note: "查找文件"},
	"Grep":  {Risk: "ok", Note: "搜索内容"},
}

// ParseAllowedTools splits the allowed-tools string on whitespace, keeping
// parenthesized argument specs (which may contain spaces) intact.
func ParseAllowedTools(raw string) []Permission {
	var out []Permission
	for _, tok := range splitTools(raw) {
		p := Permission{Tool: tok, Risk: "ok", Note: "声明的工具"}
		base := tok
		if i := strings.IndexByte(tok, '('); i > 0 {
			base = tok[:i]
		}
		if r, ok := toolRisks[base]; ok {
			p.Risk = r.Risk
			p.Note = r.Note
		}
		out = append(out, p)
	}
	return out
}

func splitTools(raw string) []string {
	var out []string
	depth := 0
	start := -1
	for i, r := range raw {
		switch {
		case r == '(':
			depth++
		case r == ')':
			if depth > 0 {
				depth--
			}
		case (r == ' ' || r == '\t' || r == '\n') && depth == 0:
			if start >= 0 {
				out = append(out, raw[start:i])
				start = -1
			}
			continue
		}
		if start < 0 && r != ' ' && r != '\t' && r != '\n' {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, raw[start:])
	}
	return out
}
