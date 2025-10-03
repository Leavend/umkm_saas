package handlers

import (
	"testing"
	"time"

	"server/internal/middleware"
)

func TestSignAndVerifyJWT(t *testing.T) {
	secret := "test-secret"
	claims := middleware.TokenClaims{
		Sub:      "user-123",
		Plan:     "free",
		Locale:   "id",
		Exp:      time.Now().Add(time.Hour).Unix(),
		Issuer:   "tester",
		Audience: "clients",
	}
	token, err := middleware.SignJWT(secret, claims)
	if err != nil {
		t.Fatalf("SignJWT() unexpected error: %v", err)
	}
	parsed, err := middleware.VerifyJWT(secret, token)
	if err != nil {
		t.Fatalf("VerifyJWT() unexpected error: %v", err)
	}
	if parsed.Sub != claims.Sub || parsed.Plan != claims.Plan || parsed.Locale != claims.Locale {
		t.Fatalf("VerifyJWT() returned %+v, want %+v", parsed, claims)
	}
}

func TestVerifyJWTInvalidSignature(t *testing.T) {
	claims := middleware.TokenClaims{
		Sub: "user-123",
		Exp: time.Now().Add(time.Hour).Unix(),
	}
	token, err := middleware.SignJWT("secret-a", claims)
	if err != nil {
		t.Fatalf("SignJWT() error: %v", err)
	}
	if _, err := middleware.VerifyJWT("secret-b", token); err == nil {
		t.Fatalf("VerifyJWT() expected invalid signature error")
	}
}

func TestVerifyJWTExpired(t *testing.T) {
	claims := middleware.TokenClaims{
		Sub: "user-123",
		Exp: time.Now().Add(-time.Minute).Unix(),
	}
	token, err := middleware.SignJWT("secret", claims)
	if err != nil {
		t.Fatalf("SignJWT() error: %v", err)
	}
	if _, err := middleware.VerifyJWT("secret", token); err == nil {
		t.Fatalf("VerifyJWT() expected expiration error")
	}
}
