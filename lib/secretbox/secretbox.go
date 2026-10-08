// Package secretbox encrypts secrets the admin stores in the database, such as
// AI provider API keys.
//
// The key comes from the SETTINGS_ENCRYPTION_KEY env var, so a database dump or
// backup on its own does not reveal the secrets in it.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Stored values carry a version prefix so the scheme can change later without
// guessing what an existing value was sealed with.
const prefix = "v1:"

var ErrMalformed = errors.New("secretbox: malformed sealed value")

type Box struct {
	aead cipher.AEAD
}

// New takes a base64-encoded 32-byte key, as produced by
// `openssl rand -base64 32`.
func New(base64Key string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Key))
	if err != nil {
		return nil, fmt.Errorf("secretbox: key is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secretbox: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts with a fresh random nonce, so sealing the same value twice
// gives different output.
func (b *Box) Seal(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Open fails on a value sealed with another key, which is what happens if
// SETTINGS_ENCRYPTION_KEY is rotated: the admin re-enters the secret.
func (b *Box) Open(sealed string) (string, error) {
	raw, ok := strings.CutPrefix(sealed, prefix)
	if !ok {
		return "", ErrMalformed
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", ErrMalformed
	}
	n := b.aead.NonceSize()
	if len(data) < n {
		return "", ErrMalformed
	}
	plain, err := b.aead.Open(nil, data[:n], data[n:], nil)
	if err != nil {
		return "", fmt.Errorf("secretbox: cannot decrypt (wrong key?): %w", err)
	}
	return string(plain), nil
}
