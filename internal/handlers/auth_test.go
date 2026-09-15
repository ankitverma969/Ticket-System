package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/auth"
	"ticket-system/internal/handlers"
	"ticket-system/internal/models"
	"ticket-system/internal/repository"
	"ticket-system/internal/service"
)

// mockUserRepo is an in-memory repository for user operations in HTTP handler tests.
type mockUserRepo struct {
	usersByEmail map[string]*models.User
	usersByID    map[string]*models.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByEmail: make(map[string]*models.User),
		usersByID:    make(map[string]*models.User),
	}
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user *models.User) error {
	norm := strings.ToLower(strings.TrimSpace(user.Email))
	if _, exists := m.usersByEmail[norm]; exists {
		return repository.ErrDuplicate
	}
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}
	user.Email = norm
	m.usersByEmail[norm] = user
	m.usersByID[user.ID.Hex()] = user
	return nil
}

func (m *mockUserRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	norm := strings.ToLower(strings.TrimSpace(email))
	u, exists := m.usersByEmail[norm]
	if !exists {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	u, exists := m.usersByID[id.Hex()]
	if !exists {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func setupAuthTestRouter() (http.Handler, *auth.TokenManager) {
	secret := "auth-handler-test-secret-12345"
	tm, _ := auth.NewTokenManager(secret, 1*time.Hour)

	userRepo := newMockUserRepo()
	authSvc := service.NewAuthService(userRepo, tm)
	handler := handlers.NewAuthHandler(authSvc)

	r := chi.NewRouter()
	r.Post("/auth/register", handler.Register)
	r.Post("/auth/login", handler.Login)

	return r, tm
}

func TestAuthHandler_RegisterAndLogin(t *testing.T) {
	router, _ := setupAuthTestRouter()

	// 1. Valid Registration -> 201 Created
	{
		reqBody := `{"email":"NewUser@example.com","password":"Password123!"}`
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["email"] != "newuser@example.com" {
			t.Errorf("expected normalized email 'newuser@example.com', got %v", resp["email"])
		}
		if _, hasPass := resp["password"]; hasPass {
			t.Errorf("SECURITY FAULT: password exposed in registration response!")
		}
		if _, hasHash := resp["password_hash"]; hasHash {
			t.Errorf("SECURITY FAULT: password_hash exposed in registration response!")
		}
	}

	// 2. Duplicate Registration -> 409 Conflict
	{
		reqBody := `{"email":"newuser@example.com","password":"Password123!"}`
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict for duplicate email, got %d", rr.Code)
		}
	}

	// 3. Malformed JSON Body -> 400 Bad Request
	{
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`{not-valid-json}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for malformed JSON, got %d", rr.Code)
		}
	}

	// 4. Missing Email -> 400 Bad Request
	{
		reqBody := `{"email":"","password":"Password123!"}`
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for missing email, got %d", rr.Code)
		}
	}

	// 5. Missing Password -> 400 Bad Request
	{
		reqBody := `{"email":"test@example.com","password":""}`
		req, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for missing password, got %d", rr.Code)
		}
	}

	// 6. Valid Login -> 200 OK
	{
		reqBody := `{"email":"NEWUSER@example.com","password":"Password123!"}`
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for valid login, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["token"] == "" || resp["token"] == nil {
			t.Errorf("expected non-empty token in login response")
		}
		if _, hasHash := resp["password_hash"]; hasHash {
			t.Errorf("SECURITY FAULT: password_hash exposed in login response!")
		}
	}

	// 7. Wrong Password -> 401 Unauthorized with generic message
	{
		reqBody := `{"email":"newuser@example.com","password":"WrongPassword!"}`
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for wrong password, got %d", rr.Code)
		}

		var errResp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &errResp)
		if errResp["error"] != "invalid credentials" {
			t.Errorf("expected generic 'invalid credentials', got %q", errResp["error"])
		}
	}

	// 8. Nonexistent User -> 401 Unauthorized with generic message
	{
		reqBody := `{"email":"nonexistent@example.com","password":"Password123!"}`
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for nonexistent user, got %d", rr.Code)
		}

		var errResp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &errResp)
		if errResp["error"] != "invalid credentials" {
			t.Errorf("expected generic 'invalid credentials', got %q", errResp["error"])
		}
	}
}
