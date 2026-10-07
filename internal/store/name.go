package store

import "regexp"

// nameRe is the artifact name rule (§9 / §10: same as the namespace rule).
var nameRe = regexp.MustCompile(`^[a-z0-9](-?[a-z0-9])*$`)

// ValidateArtifactName enforces the artifact name rule (≤64 chars,
// lowercase alnum with single hyphens).
func ValidateArtifactName(name string) bool {
	return len(name) >= 1 && len(name) <= 64 && nameRe.MatchString(name)
}
