package models

import (
	"errors"
)

// TicketStatus represents the lifecycle state of a ticket.
type TicketStatus string

const (
	StatusOpen       TicketStatus = "open"
	StatusInProgress TicketStatus = "in_progress"
	StatusClosed     TicketStatus = "closed"
)

var (
	ErrInvalidStatus     = errors.New("invalid ticket status: must be open, in_progress, or closed")
	ErrInvalidTransition = errors.New("invalid ticket status transition: must follow open -> in_progress -> closed and closed tickets cannot be reopened")
)

// IsValid checks whether the ticket status is one of the strictly allowed states.
func (s TicketStatus) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusClosed:
		return true
	default:
		return false
	}
}

// CanTransitionTo validates the mandatory business rule:
// open -> in_progress -> closed
// Closed tickets cannot be reopened.
// Retaining the same status is considered a valid no-op.
func (s TicketStatus) CanTransitionTo(target TicketStatus) bool {
	if !target.IsValid() {
		return false
	}

	if s == target {
		return true
	}

	switch s {
	case StatusOpen:
		return target == StatusInProgress
	case StatusInProgress:
		return target == StatusClosed
	case StatusClosed:
		return false // Closed ticket cannot be reopened or changed
	default:
		return false
	}
}
