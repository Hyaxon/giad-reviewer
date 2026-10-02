package githubapp

import (
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateJWT signs an App JWT using GitHub's recommended client ID issuer.
func GenerateJWT(clientID string, key *rsa.PrivateKey) (string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", fmt.Errorf("client ID is required")
	}
	if clientID != strings.TrimSpace(clientID) {
		return "", fmt.Errorf("client ID must not contain surrounding whitespace")
	}
	if key == nil {
		return "", fmt.Errorf("private key is required")
	}

	now := time.Now()

	claims := jwt.RegisteredClaims{
		Issuer:    clientID,
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign app JWT: %w", err)
	}

	return signed, nil
}
