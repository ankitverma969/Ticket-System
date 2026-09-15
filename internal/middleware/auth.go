package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/auth"
	"ticket-system/internal/response"
)

type contextKey string

const (
	userIDContextKey contextKey = "authenticated_user_id"
)

// SetUserID stores the authenticated user's ObjectID into the request context.
func SetUserID(ctx context.Context, userID bson.ObjectID) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// GetUserID retrieves the authenticated user's ObjectID from context.
// Returns false if the user ID is missing or zero.
func GetUserID(ctx context.Context) (bson.ObjectID, bool) {
	val := ctx.Value(userIDContextKey)
	if val == nil {
		return bson.NilObjectID, false
	}

	userID, ok := val.(bson.ObjectID)
	if !ok || userID.IsZero() {
		return bson.NilObjectID, false
	}

	return userID, true
}

// Auth creates an HTTP middleware that extracts and validates the Bearer JWT token,
// parses the authenticated user ObjectID, and stores it in the request context.
func Auth(tokenManager *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, http.StatusUnauthorized, "unauthorized: missing authorization header")
				return
			}

			// Validate Bearer scheme strictly
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.Error(w, http.StatusUnauthorized, "unauthorized: invalid authorization header format")
				return
			}

			tokenStr := strings.TrimSpace(parts[1])
			if tokenStr == "" {
				response.Error(w, http.StatusUnauthorized, "unauthorized: empty bearer token")
				return
			}

			// Validate token cryptographically, checking signature, alg, exp, etc.
			claims, err := tokenManager.ValidateToken(tokenStr)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "unauthorized: invalid or expired token")
				return
			}

			// Validate subject claim (must be non-empty and a valid MongoDB ObjectID hex)
			if claims.Subject == "" {
				response.Error(w, http.StatusUnauthorized, "unauthorized: missing token subject")
				return
			}

			userID, err := bson.ObjectIDFromHex(claims.Subject)
			if err != nil || userID.IsZero() {
				response.Error(w, http.StatusUnauthorized, "unauthorized: invalid user identity in token")
				return
			}

			// Inject authenticated identity into context
			ctx := SetUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
