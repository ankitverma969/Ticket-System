package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"ticket-system/internal/database"
	"ticket-system/internal/models"
)

type mongoTicketRepository struct {
	collection *mongo.Collection
}

// NewMongoTicketRepository returns a new MongoDB-backed implementation of TicketRepository.
func NewMongoTicketRepository(db *database.DB) TicketRepository {
	return &mongoTicketRepository{
		collection: db.Database.Collection(database.TicketsCollection),
	}
}

// CreateTicket persists a new ticket belonging to an authenticated user.
func (r *mongoTicketRepository) CreateTicket(ctx context.Context, ticket *models.Ticket) error {
	now := time.Now().UTC()
	ticket.CreatedAt = now
	ticket.UpdatedAt = now

	if ticket.ID.IsZero() {
		ticket.ID = bson.NewObjectID()
	}

	if ticket.Status == "" {
		ticket.Status = models.StatusOpen
	}

	_, err := r.collection.InsertOne(ctx, ticket)
	return err
}

// GetTicketsByUserID retrieves all tickets owned by the authenticated user, ordered newest first.
func (r *mongoTicketRepository) GetTicketsByUserID(ctx context.Context, userID bson.ObjectID) ([]*models.Ticket, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tickets []*models.Ticket
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, err
	}

	if tickets == nil {
		tickets = make([]*models.Ticket, 0)
	}

	return tickets, nil
}

// GetTicketByIDAndUserID retrieves a single ticket ensuring it belongs to the authenticated user.
// This fulfills the core assignment requirement: "Users must only be able to view tickets created by themselves".
func (r *mongoTicketRepository) GetTicketByIDAndUserID(ctx context.Context, ticketID, userID bson.ObjectID) (*models.Ticket, error) {
	var ticket models.Ticket
	filter := bson.M{
		"_id":     ticketID,
		"user_id": userID,
	}

	err := r.collection.FindOne(ctx, filter).Decode(&ticket)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &ticket, nil
}

// UpdateTicketStatus updates the status of a ticket owned by the authenticated user.
func (r *mongoTicketRepository) UpdateTicketStatus(ctx context.Context, ticketID, userID bson.ObjectID, status models.TicketStatus) error {
	filter := bson.M{
		"_id":     ticketID,
		"user_id": userID,
	}

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateTicketStatusAtomic conditionally updates a ticket status only if it matches expectedCurrent.
// This prevents race conditions and state machine violations from concurrent requests.
func (r *mongoTicketRepository) UpdateTicketStatusAtomic(ctx context.Context, ticketID, userID bson.ObjectID, expectedCurrent models.TicketStatus, newStatus models.TicketStatus) error {
	filter := bson.M{
		"_id":     ticketID,
		"user_id": userID,
		"status":  expectedCurrent,
	}

	update := bson.M{
		"$set": bson.M{
			"status":     newStatus,
			"updated_at": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}
