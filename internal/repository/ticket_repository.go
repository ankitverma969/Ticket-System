package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/models"
)

// TicketRepository defines the persistence contract for ticket operations.
// Crucially, query methods require userID to ensure ownership isolation at the repository level.
type TicketRepository interface {
	CreateTicket(ctx context.Context, ticket *models.Ticket) error
	GetTicketsByUserID(ctx context.Context, userID bson.ObjectID) ([]*models.Ticket, error)
	GetTicketByIDAndUserID(ctx context.Context, ticketID, userID bson.ObjectID) (*models.Ticket, error)
	UpdateTicketStatus(ctx context.Context, ticketID, userID bson.ObjectID, status models.TicketStatus) error
}
