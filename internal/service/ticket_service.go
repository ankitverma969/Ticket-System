package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/models"
	"ticket-system/internal/repository"
)

var (
	ErrEmptyTitle       = errors.New("ticket title cannot be empty")
	ErrTitleTooLong     = errors.New("ticket title exceeds maximum length of 200 characters")
	ErrEmptyDescription = errors.New("ticket description cannot be empty")
	ErrDescTooLong      = errors.New("ticket description exceeds maximum length of 5000 characters")
	ErrUnauthorized     = errors.New("unauthorized")
)

const (
	MaxTitleLength       = 200
	MaxDescriptionLength = 5000
)

// CreateTicketInput represents the payload submitted by the client.
// Note: user_id and status are intentionally not bound or trusted to control ownership or status.
type CreateTicketInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// TicketOutput represents the public JSON representation of a ticket.
type TicketOutput struct {
	ID          string              `json:"id"`
	UserID      string              `json:"user_id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Status      models.TicketStatus `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// TicketService defines business operations for tickets.
type TicketService interface {
	CreateTicket(ctx context.Context, userID bson.ObjectID, input CreateTicketInput) (*TicketOutput, error)
	GetTicketsByUserID(ctx context.Context, userID bson.ObjectID) ([]*TicketOutput, error)
}

type ticketService struct {
	ticketRepo repository.TicketRepository
}

// NewTicketService creates a new TicketService instance.
func NewTicketService(ticketRepo repository.TicketRepository) TicketService {
	return &ticketService{
		ticketRepo: ticketRepo,
	}
}

func (s *ticketService) CreateTicket(ctx context.Context, userID bson.ObjectID, input CreateTicketInput) (*TicketOutput, error) {
	if userID.IsZero() {
		return nil, ErrUnauthorized
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, ErrEmptyTitle
	}
	if len(title) > MaxTitleLength {
		return nil, ErrTitleTooLong
	}

	desc := strings.TrimSpace(input.Description)
	if desc == "" {
		return nil, ErrEmptyDescription
	}
	if len(desc) > MaxDescriptionLength {
		return nil, ErrDescTooLong
	}

	ticket := &models.Ticket{
		UserID:      userID, // Ownership is enforced strictly from authenticated context
		Title:       title,
		Description: desc,
		Status:      models.StatusOpen, // New tickets MUST start as "open"
	}

	if err := s.ticketRepo.CreateTicket(ctx, ticket); err != nil {
		return nil, err
	}

	return toTicketOutput(ticket), nil
}

func (s *ticketService) GetTicketsByUserID(ctx context.Context, userID bson.ObjectID) ([]*TicketOutput, error) {
	if userID.IsZero() {
		return nil, ErrUnauthorized
	}

	tickets, err := s.ticketRepo.GetTicketsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	outputs := make([]*TicketOutput, 0, len(tickets))
	for _, t := range tickets {
		outputs = append(outputs, toTicketOutput(t))
	}

	return outputs, nil
}

func toTicketOutput(t *models.Ticket) *TicketOutput {
	return &TicketOutput{
		ID:          t.ID.Hex(),
		UserID:      t.UserID.Hex(),
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
