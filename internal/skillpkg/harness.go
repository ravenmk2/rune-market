package skillpkg

import (
	"path"
	"strings"
)

// Harness IDs from the §9 detection table; empty means generic.
const (
	HarnessClaudeCode = "claude-code"
	HarnessCodex      = "codex"
	HarnessCursor     = "cursor"
)

// claudeCodeFields: presence of any marks the skill as claude-code (§9).
var claudeCodeFields = []string{
	"hooks", "model", "context", "agent", "argument-hint", "user-invocable",
	"disable-model-invocation", "disallowed-tools", "when_to_use", "shell",
	"background", "effort", "arguments",
}

// cursorFields: presence of any marks the skill as cursor (§9).
var cursorFields = []string{"icon", "color", "paths"}

// detectHarnesses applies the §9 rule table. entries are archive-relative
// paths; rootPrefix is the located skill directory ("" for root form).
func detectHarnesses(frontmatter map[string]any, files []FileEntry, rootPrefix string) []string {
	var out []string
	if hasAny(frontmatter, claudeCodeFields) {
		out = append(out, HarnessClaudeCode)
	}
	openaiYAML := path.Join(rootPrefix, "agents/openai.yaml")
	for _, f := range files {
		if f.Path == openaiYAML {
			out = append(out, HarnessCodex)
			break
		}
	}
	if hasAny(frontmatter, cursorFields) {
		out = append(out, HarnessCursor)
	}
	return out
}

func hasAny(fm map[string]any, keys []string) bool {
	for _, k := range keys {
		if _, ok := fm[k]; ok {
			return true
		}
	}
	return false
}

// extensionFields returns frontmatter keys outside the spec 6 fields.
func extensionFields(fm map[string]any) []string {
	var out []string
	for k := range fm {
		if !specFields[k] {
			out = append(out, k)
		}
	}
	return out
}

var specFields = map[string]bool{
	"name": true, "description": true, "compatibility": true,
	"license": true, "allowed-tools": true, "metadata": true,
}

// sortedKeys is a tiny helper to keep report output deterministic.
func sortedStrings(in []string) []string {
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && strings.Compare(in[j-1], in[j]) > 0; j-- {
			in[j-1], in[j] = in[j], in[j-1]
		}
	}
	return in
}
