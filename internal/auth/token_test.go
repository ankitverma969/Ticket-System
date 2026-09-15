package auth_test

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/auth"
)

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	secret := "mysecretkey"
	tm, err := auth.NewTokenManager(secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("expected NewTokenManager to succeed, got: %v", err)
	}

	userID := bson.NewObjectID()
	token, err := tm.GenerateToken(userID)
	if err != nil {
		t.Fatalf("expected GenerateToken to succeed, got: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token to be non-empty")
	}

	// Validate valid token
	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("expected ValidateToken to succeed, got: %v", err)
	}

	if claims.Subject != userID.Hex() {
		t.Errorf("expected subject %q, got %q", userID.Hex(), claims.Subject)
	}

	// Validation with wrong secret fails
	wrongTM, _ := auth.NewTokenManager("wrongsecret", 1*time.Hour)
	_, err = wrongTM.ValidateToken(token)
	if err == nil {
		t.Errorf("expected validation with wrong secret to fail")
	}

	// Expired token test
	expiredTM, _ := auth.NewTokenManager(secret, -1*time.Hour)
	expiredToken, _ := expiredTM.GenerateToken(userID)
	_, err = tm.ValidateToken(expiredToken)
	if err != auth.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got: %v", err)
	}
}
