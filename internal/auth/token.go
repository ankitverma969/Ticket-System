package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	ErrInvalidToken = errors.New("invalid or malformed token")
	ErrExpiredToken = errors.New("token has expired")
)

// Claims represents the JWT claims payload including standard registered claims.
type Claims struct {
	jwt.RegisteredClaims
}

// TokenManager handles generation and validation of JWT tokens.
type TokenManager struct {
	secret     []byte
	expiration time.Duration
}

// NewTokenManager creates a new TokenManager.
// Returns an error if secret is empty.
func NewTokenManager(secret string, expiration time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("jwt secret cannot be empty")
	}
	if expiration == 0 {
		expiration = 24 * time.Hour
	}

	return &TokenManager{
		secret:     []byte(secret),
		expiration: expiration,
	}, nil
}

// GenerateToken creates a signed HS256 JWT containing the user's ObjectID as the subject ('sub').
func (tm *TokenManager) GenerateToken(userID bson.ObjectID) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.Hex(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.expiration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(tm.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, nil
}

// ValidateToken parses and validates a signed JWT token string.
// Returns the Claims if valid, or an appropriate error.
func (tm *TokenManager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return tm.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
