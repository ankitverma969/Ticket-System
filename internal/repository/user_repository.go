package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/models"
)

// UserRepository defines the persistence contract for user operations.
type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id bson.ObjectID) (*models.User, error)
}
