package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCE — Proof Key for Code Exchange (RFC 7636)
// Protegge il flusso Authorization Code senza client_secret.

type PKCEPair struct {
	Verifier  string // segreto locale, non esce mai dalla CLI
	Challenge string // SHA256(verifier) in base64url, mandato a Keycloak
}

// NewPKCE genera una coppia verifier/challenge.
func NewPKCE() (*PKCEPair, error) {
	// 64 bytes random → 86 caratteri base64url (ben oltre il minimo RFC 43)
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}

	verifier := base64.RawURLEncoding.EncodeToString(buf)

	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	return &PKCEPair{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}
