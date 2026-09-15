package service

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"ticket-system/internal/auth"
	"ticket-system/internal/models"
	"ticket-system/internal/repository"
)

var (
	ErrInvalidEmail       = errors.New("invalid or empty email")
	ErrInvalidPassword    = errors.New("password cannot be empty")
	ErrPasswordTooLong    = errors.New("password exceeds maximum allowed length")
	ErrDuplicateEmail     = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

const maxPasswordLength = 72 // bcrypt maximum effective length

// RegisterInput represents input payload for user registration.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginInput represents input payload for user login.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UserOutput represents safe public user information without password or hashes.
type UserOutput struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// AuthOutput represents successful authentication with JWT token.
type AuthOutput struct {
	Token string     `json:"token"`
	User  UserOutput `json:"user"`
}

// AuthService defines authentication business operations.
type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*UserOutput, error)
	Login(ctx context.Context, input LoginInput) (*AuthOutput, error)
}

type authService struct {
	userRepo     repository.UserRepository
	tokenManager *auth.TokenManager
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(userRepo repository.UserRepository, tokenManager *auth.TokenManager) AuthService {
	return &authService{
		userRepo:     userRepo,
		tokenManager: tokenManager,
	}
}

// NormalizeEmail trims whitespace and converts email to lowercase consistently.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *authService) Register(ctx context.Context, input RegisterInput) (*UserOutput, error) {
	normEmail := NormalizeEmail(input.Email)
	if normEmail == "" || !strings.Contains(normEmail, "@") {
		return nil, ErrInvalidEmail
	}

	if strings.TrimSpace(input.Password) == "" {
		return nil, ErrInvalidPassword
	}

	if len(input.Password) > maxPasswordLength {
		return nil, ErrPasswordTooLong
	}

	// Application-level check
	existing, err := s.userRepo.GetUserByEmail(ctx, normEmail)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateEmail
	}

	// Hash password using bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        normEmail,
		PasswordHash: string(hash),
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}

	return &UserOutput{
		ID:    user.ID.Hex(),
		Email: user.Email,
	}, nil
}

func (s *authService) Login(ctx context.Context, input LoginInput) (*AuthOutput, error) {
	normEmail := NormalizeEmail(input.Email)
	if normEmail == "" || strings.TrimSpace(input.Password) == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.userRepo.GetUserByEmail(ctx, normEmail)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Do not leak whether email exists
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Compare bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokenManager.GenerateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthOutput{
		Token: token,
		User: UserOutput{
			ID:    user.ID.Hex(),
			Email: user.Email,
		},
	}, nil
}
