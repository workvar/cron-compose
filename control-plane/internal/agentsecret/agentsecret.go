// Package agentsecret creates and checks the per-server secret an agent presents when
// it connects through an edge that ends TLS and so cannot pass a client certificate.
// Only a SHA-256 of the secret is stored, so a leaked database does not leak logins.
package agentsecret

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

// Generate returns a new random secret and the hash to store for it.
func Generate() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	return plain, Hash(plain), nil
}

// Hash returns the hex SHA-256 of a secret.
func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// Matches reports whether plain hashes to stored, in constant time. An empty stored
// hash never matches, so a server that was never given a secret cannot be logged in
// with an empty one.
func Matches(plain, stored string) bool {
	if plain == "" || stored == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(Hash(plain)), []byte(stored)) == 1
}
