// Package auth implements the OAuth 2.1 Authorization Code + PKCE flow for a
// public client, as required by the Path of Exile API for desktop applications.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCE holds a code verifier and its derived S256 challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE generates a fresh code verifier (32 random bytes, base64url-encoded)
// and its SHA-256 challenge, per RFC 7636. GGG requires the S256 method.
func NewPKCE() (PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: verifier, Challenge: challenge}, nil
}

// randomState returns a URL-safe random string for the OAuth `state` parameter
// (CSRF protection).
func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
