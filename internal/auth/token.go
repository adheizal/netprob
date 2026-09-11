package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// GenerateToken generates a random token for agent authentication.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken hashes a token using SHA-256 for storage.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// VerifyToken checks if a token matches the stored hash.
func VerifyToken(token, storedHash string) bool {
	h := HashToken(token)
	return h == storedHash
}

// TokenDuration is how long an agent token is valid.
const TokenDuration = 0 // tokens don't expire for MVP, but we keep the constant for future use

// IsExpired checks if a token was issued before the given time.
func IsExpired(issuedAt time.Time, duration time.Duration) bool {
	if duration == 0 {
		return false
	}
	return time.Since(issuedAt) > duration
}
