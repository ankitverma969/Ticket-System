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

// CanTransitionTo strictly validates the mandatory state machine flow:
// open -> in_progress
// in_progress -> closed
// All other transitions (including same-status no-ops, jumps like open->closed, backward transitions,
// and any transition from closed) are strictly invalid.
func (s TicketStatus) CanTransitionTo(target TicketStatus) bool {
	if !target.IsValid() {
		return false
	}

	switch s {
	case StatusOpen:
		return target == StatusInProgress
	case StatusInProgress:
		return target == StatusClosed
	case StatusClosed:
		return false // Closed ticket can NEVER be reopened or transitioned
	default:
		return false
	}
}
