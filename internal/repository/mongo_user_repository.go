package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"ticket-system/internal/database"
	"ticket-system/internal/models"
)

type mongoUserRepository struct {
	collection *mongo.Collection
}

// NewMongoUserRepository returns a new MongoDB-backed implementation of UserRepository.
func NewMongoUserRepository(db *database.DB) UserRepository {
	return &mongoUserRepository{
		collection: db.Database.Collection(database.UsersCollection),
	}
}

// CreateUser persists a new user into MongoDB with timestamps and normalized email.
func (r *mongoUserRepository) CreateUser(ctx context.Context, user *models.User) error {
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicate
		}
		return err
	}

	return nil
}

// GetUserByEmail queries a user by normalized email.
func (r *mongoUserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	var user models.User

	err := r.collection.FindOne(ctx, bson.M{"email": normEmail}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByID queries a user by their unique ObjectID.
func (r *mongoUserRepository) GetUserByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	var user models.User

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &user, nil
}
