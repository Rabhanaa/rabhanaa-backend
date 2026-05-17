package service

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const DefaultSignupSource = "direct"

var allowedSignupSources = map[string]struct{}{
	"facebook":  {},
	"google":    {},
	"instagram": {},
	"tiktok":    {},
	"x":         {},
	"snapchat":  {},
	"friend":    {},
	"app_store": {},
	"search":    {},
	"other":     {},
	"direct":    {},
}

// NormalizeSignupSource trims and lowercases the input, returning the default
// when empty. It does not validate; pair it with IsValidSignupSource.
func NormalizeSignupSource(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return DefaultSignupSource
	}
	return s
}

func IsValidSignupSource(s string) bool {
	_, ok := allowedSignupSources[s]
	return ok
}

var phoneRegex = regexp.MustCompile(`^01[0125]\d{8}$`)

func ValidatePhone(phone string) bool {
	return phoneRegex.MatchString(phone)
}

func ValidatePassword(password string) bool {
	if len(password) < 8 || len(password) > 16 {
		return false
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}
	return hasUpper && hasLower && hasDigit && hasSpecial
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
