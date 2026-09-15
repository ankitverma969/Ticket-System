package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/auth"
	"ticket-system/internal/models"
	"ticket-system/internal/repository"
	"ticket-system/internal/service"
)

// mockUserRepo is an in-memory implementation of UserRepository for unit testing.
type mockUserRepo struct {
	usersByID    map[string]*models.User
	usersByEmail map[string]*models.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByID:    make(map[string]*models.User),
		usersByEmail: make(map[string]*models.User),
	}
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user *models.User) error {
	normEmail := strings.ToLower(strings.TrimSpace(user.Email))
	if _, exists := m.usersByEmail[normEmail]; exists {
		return repository.ErrDuplicate
	}
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}
	user.Email = normEmail
	m.usersByID[user.ID.Hex()] = user
	m.usersByEmail[normEmail] = user
	return nil
}

func (m *mockUserRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	u, ok := m.usersByEmail[normEmail]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	u, ok := m.usersByID[id.Hex()]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func TestAuthService_RegisterAndLogin(t *testing.T) {
	tm, _ := auth.NewTokenManager("testsecret123", 1*time.Hour)
	repo := newMockUserRepo()
	svc := service.NewAuthService(repo, tm)
	ctx := context.Background()

	// 1. Successful registration
	regOut, err := svc.Register(ctx, service.RegisterInput{
		Email:    "TestUser@Domain.COM",
		Password: "SecurePassword123!",
	})
	if err != nil {
		t.Fatalf("expected registration to succeed, got: %v", err)
	}

	if regOut.Email != "testuser@domain.com" {
		t.Errorf("expected normalized email 'testuser@domain.com', got %q", regOut.Email)
	}

	// Verify password is stored hashed, NOT plaintext
	stored, err := repo.GetUserByEmail(ctx, "testuser@domain.com")
	if err != nil {
		t.Fatalf("failed to fetch stored user: %v", err)
	}
	if stored.PasswordHash == "SecurePassword123!" {
		t.Errorf("password was stored in plaintext!")
	}
	if !strings.HasPrefix(stored.PasswordHash, "$2a$") && !strings.HasPrefix(stored.PasswordHash, "$2b$") {
		t.Errorf("password hash does not appear to be bcrypt formatted: %s", stored.PasswordHash)
	}

	// 2. Duplicate email rejected
	_, err = svc.Register(ctx, service.RegisterInput{
		Email:    "testuser@domain.com",
		Password: "otherpassword",
	})
	if err != service.ErrDuplicateEmail {
		t.Errorf("expected ErrDuplicateEmail, got: %v", err)
	}

	// 3. Validation errors
	_, err = svc.Register(ctx, service.RegisterInput{Email: "", Password: "valid"})
	if err != service.ErrInvalidEmail {
		t.Errorf("expected ErrInvalidEmail for empty email, got: %v", err)
	}

	_, err = svc.Register(ctx, service.RegisterInput{Email: "user@domain.com", Password: ""})
	if err != service.ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword for empty password, got: %v", err)
	}

	// 4. Successful login
	loginOut, err := svc.Login(ctx, service.LoginInput{
		Email:    "TESTUSER@domain.com",
		Password: "SecurePassword123!",
	})
	if err != nil {
		t.Fatalf("expected login to succeed, got: %v", err)
	}
	if loginOut.Token == "" {
		t.Errorf("expected non-empty token")
	}
	if loginOut.User.ID != stored.ID.Hex() {
		t.Errorf("expected user ID %q, got %q", stored.ID.Hex(), loginOut.User.ID)
	}

	// 5. Wrong password fails with ErrInvalidCredentials
	_, err = svc.Login(ctx, service.LoginInput{
		Email:    "testuser@domain.com",
		Password: "WrongPassword",
	})
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}

	// 6. Unknown email fails with generic ErrInvalidCredentials
	_, err = svc.Login(ctx, service.LoginInput{
		Email:    "unknown@domain.com",
		Password: "SomePassword",
	})
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for unknown user, got: %v", err)
	}
}
