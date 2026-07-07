package enterprise

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// keyByteLen is the raw entropy of a generated API key before encoding.
const keyByteLen = 24

// keyPrefixLen is how many leading chars of the raw key we persist in cleartext
// so the UI can show which key is which (the rest is only ever stored hashed).
const keyPrefixLen = 8

// generateKey returns a fresh random API key in the form "ivyk_<hex>". The raw
// key is shown to the caller exactly once; only its SHA-256 hash is stored.
func generateKey() (raw string, err error) {
	b := make([]byte, keyByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ivyk_" + hex.EncodeToString(b), nil
}

// hashKey returns the hex SHA-256 of a raw key. Lookups query by this hash so
// the plaintext key never touches the database.
func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// prefixOf returns the first keyPrefixLen chars of the raw key, or the whole
// string if it is shorter.
func prefixOf(raw string) string {
	if len(raw) <= keyPrefixLen {
		return raw
	}
	return raw[:keyPrefixLen]
}

// constantTimeEqual compares two hex hashes without leaking timing information.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
