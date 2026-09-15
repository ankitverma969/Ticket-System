package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/auth"
	"ticket-system/internal/middleware"
	"ticket-system/internal/response"
)

func TestAuthMiddleware(t *testing.T) {
	secret := "test-secret-key-12345"
	tm, err := auth.NewTokenManager(secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to create TokenManager: %v", err)
	}

	validUserID := bson.NewObjectID()
	validToken, err := tm.GenerateToken(validUserID)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}

	// Token with expired time
	expiredTM, _ := auth.NewTokenManager(secret, -1*time.Hour)
	expiredToken, _ := expiredTM.GenerateToken(validUserID)

	// Token signed with a different secret
	wrongSecretTM, _ := auth.NewTokenManager("wrong-secret-key-99999", 1*time.Hour)
	wrongSecretToken, _ := wrongSecretTM.GenerateToken(validUserID)

	// Token with invalid/empty subject
	invalidSubClaims := auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "not-a-valid-hex-id",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	invalidSubJwt := jwt.NewWithClaims(jwt.SigningMethodHS256, invalidSubClaims)
	invalidSubToken, _ := invalidSubJwt.SignedString([]byte(secret))

	// Protected test handler verifying context identity
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserID(r.Context())
		if !ok || userID.IsZero() {
			response.Error(w, http.StatusUnauthorized, "no user in context")
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"user_id": userID.Hex()})
	})

	authMiddleware := middleware.Auth(tm)
	protectedRoute := authMiddleware(testHandler)

	tests := []struct {
		name               string
		authHeader         string
		expectedStatusCode int
		expectUserIDMatch  bool
	}{
		{
			name:               "Valid token succeeds and sets user ID in context",
			authHeader:         "Bearer " + validToken,
			expectedStatusCode: http.StatusOK,
			expectUserIDMatch:  true,
		},
		{
			name:               "Missing authorization header",
			authHeader:         "",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Missing Bearer scheme (token only)",
			authHeader:         validToken,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Wrong scheme (Basic)",
			authHeader:         "Basic " + validToken,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Bearer with empty token",
			authHeader:         "Bearer ",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Malformed token",
			authHeader:         "Bearer abc.xyz.invalid",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Expired token",
			authHeader:         "Bearer " + expiredToken,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Wrong signing secret",
			authHeader:         "Bearer " + wrongSecretToken,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Invalid ObjectID subject",
			authHeader:         "Bearer " + invalidSubToken,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Tampered token payload",
			authHeader:         "Bearer " + validToken + "tampered",
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/protected-test", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			protectedRoute.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatusCode {
				t.Errorf("%s: expected status %d, got %d. Body: %s", tt.name, tt.expectedStatusCode, rr.Code, rr.Body.String())
			}

			// Verify application/json content type on failure and success
			ct := rr.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("%s: expected Content-Type application/json, got %q", tt.name, ct)
			}

			if tt.expectUserIDMatch {
				var data map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
					t.Fatalf("failed to decode response JSON: %v", err)
				}
				if data["user_id"] != validUserID.Hex() {
					t.Errorf("expected user_id %q, got %q", validUserID.Hex(), data["user_id"])
				}
			}
		})
	}
}
