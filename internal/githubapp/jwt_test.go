package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().Truncate(time.Second)
	signed, err := GenerateJWT("Iv1.example", key)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now()
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(signed, claims, func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer("Iv1.example"), jwt.WithExpirationRequired())
	if err != nil {
		t.Fatalf("verify signed JWT: %v", err)
	}
	if !token.Valid {
		t.Fatal("JWT is not valid")
	}
	if claims.IssuedAt == nil || claims.IssuedAt.Time.Before(before.Add(-time.Minute)) || claims.IssuedAt.Time.After(after.Add(-time.Minute)) {
		t.Fatal("expected issue time backdated one minute")
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(before.Add(9*time.Minute)) || claims.ExpiresAt.Time.After(after.Add(9*time.Minute)) {
		t.Fatal("expected expiration nine minutes in the future")
	}
}
