package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"ticket-system/internal/config"
)

// Centralized collection names to avoid scattering magic strings across packages.
const (
	UsersCollection   = "users"
	TicketsCollection = "tickets"
)

// DB encapsulates the connected MongoDB client and targeted database instance.
type DB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// Connect establishes a connection to MongoDB using the official v2 driver, pings
// the server to verify connectivity, and returns the DB wrapper.
func Connect(ctx context.Context, cfg *config.Config) (*DB, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoDBURI)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create mongodb client: %w", err)
	}

	// Ping MongoDB with a strict timeout to verify connectivity immediately
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	db := client.Database(cfg.MongoDBDatabase)

	return &DB{
		Client:   client,
		Database: db,
	}, nil
}

// Disconnect cleanly closes the MongoDB client connection.
func (d *DB) Disconnect(ctx context.Context) error {
	if d.Client != nil {
		return d.Client.Disconnect(ctx)
	}
	return nil
}

// CreateIndexes ensures required indexes are established for performance and uniqueness constraints:
// 1. users.email -> UNIQUE index (prevents duplicate registrations)
// 2. tickets.user_id -> index (optimizes querying tickets owned by a specific user)
func (d *DB) CreateIndexes(ctx context.Context) error {
	// Index for users collection: unique email
	usersColl := d.Database.Collection(UsersCollection)
	emailIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := usersColl.Indexes().CreateOne(ctx, emailIndex)
	if err != nil {
		return fmt.Errorf("failed to create unique index on users.email: %w", err)
	}

	// Index for tickets collection: user_id index for fast lookup of tickets by owner
	ticketsColl := d.Database.Collection(TicketsCollection)
	userIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}},
	}
	_, err = ticketsColl.Indexes().CreateOne(ctx, userIndex)
	if err != nil {
		return fmt.Errorf("failed to create index on tickets.user_id: %w", err)
	}

	return nil
}
