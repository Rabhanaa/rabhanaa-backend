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

// NormalizeEmail lowercases and trims an address before it is stored or looked
// up.
//
// Email domains are case-insensitive and no mail provider in practice treats the
// local part as case-sensitive, but the users.email unique index is. Without
// this, registering "X@Gmail.com" when "x@gmail.com" exists passes the
// duplicate check and then dies on the index — surfacing as an unexplained
// "unexpected error" rather than "this address is taken". Worse, an account
// stored with capitals can never log in with the address its owner types, and
// its password reset silently does nothing, because that endpoint reports
// success whether or not the address was found.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
