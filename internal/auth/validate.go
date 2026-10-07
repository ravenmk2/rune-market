package auth

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// usernameRe is the namespace rule from design §7; it also forbids
// consecutive hyphens by construction.
var usernameRe = regexp.MustCompile(`^[a-z0-9](-?[a-z0-9])*$`)

func ValidateUsername(username string) error {
	if utf8.RuneCountInString(username) > 64 {
		return errors.New("username must be at most 64 characters")
	}
	if !usernameRe.MatchString(username) {
		return errors.New("username must match ^[a-z0-9](-?[a-z0-9])*$")
	}
	return nil
}

func ValidateNickname(nickname string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(nickname))
	if n == 0 {
		return errors.New("nickname is required")
	}
	if n > 64 {
		return errors.New("nickname must be at most 64 characters")
	}
	return nil
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	// bcrypt silently truncates at 72 bytes; reject instead of surprising users
	if len(password) > 72 {
		return errors.New("password must be at most 72 bytes")
	}
	return nil
}

// ValidateCredentials aggregates all registration field errors.
func ValidateCredentials(username, nickname, password string) (details []string) {
	if err := ValidateUsername(username); err != nil {
		details = append(details, fmt.Sprintf("username: %s", err))
	}
	if err := ValidateNickname(nickname); err != nil {
		details = append(details, fmt.Sprintf("nickname: %s", err))
	}
	if err := ValidatePassword(password); err != nil {
		details = append(details, fmt.Sprintf("password: %s", err))
	}
	return details
}

// HashPassword hashes with bcrypt at the design default cost (§7).
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
