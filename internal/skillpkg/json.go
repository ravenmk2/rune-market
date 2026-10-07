package skillpkg

import "encoding/json"

// toJSON marshals the frontmatter map for DB passthrough (§9.7).
// Keys are sorted by encoding/json; nil maps marshal as {}.
func toJSON(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
