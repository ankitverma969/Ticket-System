package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Ticket represents a support ticket in MongoDB and service layers.
// Ticket.UserID explicitly links to User.ID for ownership enforcement.
type Ticket struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      bson.ObjectID `bson:"user_id" json:"user_id"`
	Title       string        `bson:"title" json:"title"`
	Description string        `bson:"description" json:"description"`
	Status      TicketStatus  `bson:"status" json:"status"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updated_at"`
}
