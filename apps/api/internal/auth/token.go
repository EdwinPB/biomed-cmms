package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewToken returns a fresh opaque session token. The token is 32 random bytes
// encoded as hex (64 characters); it is given to the client once and never
// stored in plaintext.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 digest of a session token, hex encoded. Only
// the digest is persisted in the database.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
