package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNewPKCE(t *testing.T) {
	p, err := NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if p.Verifier == "" || p.Challenge == "" {
		t.Fatal("empty pkce values")
	}
	// Challenge must be base64url(sha256(verifier)) per RFC 7636 S256.
	sum := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.Challenge != want {
		t.Errorf("challenge = %q, want %q", p.Challenge, want)
	}
}

func TestPKCEUnique(t *testing.T) {
	a, _ := NewPKCE()
	b, _ := NewPKCE()
	if a.Verifier == b.Verifier {
		t.Error("verifiers should be unique")
	}
}

func TestTokenExpired(t *testing.T) {
	if !(Token{}).Expired(0) {
		t.Error("empty token should be expired")
	}
}
