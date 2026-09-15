package models_test

import (
	"testing"

	"ticket-system/internal/models"
)

func TestTicketStatusIsValid(t *testing.T) {
	tests := []struct {
		status models.TicketStatus
		valid  bool
	}{
		{models.StatusOpen, true},
		{models.StatusInProgress, true},
		{models.StatusClosed, true},
		{models.TicketStatus("pending"), false},
		{models.TicketStatus(""), false},
		{models.TicketStatus("OPEN"), false},
		{models.TicketStatus("resolved"), false},
	}

	for _, tt := range tests {
		if got := tt.status.IsValid(); got != tt.valid {
			t.Errorf("TicketStatus(%q).IsValid() = %v; expected %v", tt.status, got, tt.valid)
		}
	}
}

func TestTicketStatusTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     models.TicketStatus
		to       models.TicketStatus
		expected bool
	}{
		// Valid transitions
		{"open -> in_progress (valid)", models.StatusOpen, models.StatusInProgress, true},
		{"in_progress -> closed (valid)", models.StatusInProgress, models.StatusClosed, true},

		// Same-status no-ops
		{"open -> open (no-op)", models.StatusOpen, models.StatusOpen, true},
		{"in_progress -> in_progress (no-op)", models.StatusInProgress, models.StatusInProgress, true},
		{"closed -> closed (no-op)", models.StatusClosed, models.StatusClosed, true},

		// Invalid transitions
		{"open -> closed (invalid jump)", models.StatusOpen, models.StatusClosed, false},
		{"in_progress -> open (backward)", models.StatusInProgress, models.StatusOpen, false},
		{"closed -> open (closed cannot be reopened)", models.StatusClosed, models.StatusOpen, false},
		{"closed -> in_progress (closed cannot be changed)", models.StatusClosed, models.StatusInProgress, false},

		// Invalid target states
		{"open -> unknown", models.StatusOpen, models.TicketStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.expected {
				t.Errorf("%s: CanTransitionTo(%q) = %v, expected %v", tt.name, tt.to, got, tt.expected)
			}
		})
	}
}
