package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// User represents an application user in MongoDB and service layers.
// Security: PasswordHash is tagged with `json:"-"` so it is NEVER serialized in JSON API responses.
type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string        `bson:"email" json:"email"`
	PasswordHash string        `bson:"password_hash" json:"-"`
	CreatedAt    time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time     `bson:"updated_at" json:"updated_at"`
}
