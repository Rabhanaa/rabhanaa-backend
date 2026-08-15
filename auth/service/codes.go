package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
)

// generateCode returns a 6-digit numeric code.
//
// crypto/rand, not math/rand: this now backs password reset, and a generator
// seeded from the clock is predictable to anyone who can guess when the request
// was made.
func generateCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		// crypto/rand only fails if the system entropy source is broken, in
		// which case issuing a guessable code is worse than failing loudly.
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// checkCode compares in constant time so a wrong code cannot be narrowed down
// by timing the response.
func checkCode(code, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(hashCode(code)), []byte(hash)) == 1
}
