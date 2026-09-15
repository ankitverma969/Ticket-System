package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/middleware"
	"ticket-system/internal/response"
	"ticket-system/internal/service"
)

// TicketHandler handles HTTP requests for ticket operations.
type TicketHandler struct {
	ticketService service.TicketService
}

// NewTicketHandler creates a new TicketHandler instance.
func NewTicketHandler(ticketService service.TicketService) *TicketHandler {
	return &TicketHandler{
		ticketService: ticketService,
	}
}

// Create handles POST /tickets.
func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID.IsZero() {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input service.CreateTicketInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "malformed or invalid JSON body")
		return
	}

	ticket, err := h.ticketService.CreateTicket(r.Context(), userID, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmptyTitle),
			errors.Is(err, service.ErrTitleTooLong),
			errors.Is(err, service.ErrEmptyDescription),
			errors.Is(err, service.ErrDescTooLong):
			response.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrUnauthorized):
			response.Error(w, http.StatusUnauthorized, "unauthorized")
		default:
			response.Error(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.JSON(w, http.StatusCreated, ticket)
}

// List handles GET /tickets (returns only the authenticated user's tickets).
func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID.IsZero() {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tickets, err := h.ticketService.GetTicketsByUserID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		response.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.JSON(w, http.StatusOK, tickets)
}

// GetByID handles GET /tickets/{id}.
// Returns the ticket if it belongs to the authenticated user.
// Returns 400 for malformed/invalid ticket ID.
// Returns 404 if ticket doesn't exist OR belongs to another user (preventing information leakage).
func (h *TicketHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID.IsZero() {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idParam := chi.URLParam(r, "id")
	ticketID, err := bson.ObjectIDFromHex(idParam)
	if err != nil || ticketID.IsZero() {
		response.Error(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	ticket, err := h.ticketService.GetTicketByIDAndUserID(r.Context(), ticketID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTicketNotFound):
			response.Error(w, http.StatusNotFound, "ticket not found")
		case errors.Is(err, service.ErrUnauthorized):
			response.Error(w, http.StatusUnauthorized, "unauthorized")
		default:
			response.Error(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.JSON(w, http.StatusOK, ticket)
}

// UpdateStatus handles PATCH /tickets/{id}/status.
// Allows updating status strictly following open -> in_progress -> closed.
// Returns 400 for missing/malformed JSON, invalid ticket ID, or invalid status string.
// Returns 401 for missing/invalid JWT.
// Returns 404 if ticket doesn't exist or doesn't belong to the authenticated user.
// Returns 409 Conflict if the status transition is disallowed by state machine rules.
func (h *TicketHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok || userID.IsZero() {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idParam := chi.URLParam(r, "id")
	ticketID, err := bson.ObjectIDFromHex(idParam)
	if err != nil || ticketID.IsZero() {
		response.Error(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	var input service.UpdateTicketStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "malformed or invalid JSON body")
		return
	}

	if strings.TrimSpace(input.Status) == "" {
		response.Error(w, http.StatusBadRequest, "missing status field")
		return
	}

	updated, err := h.ticketService.UpdateTicketStatus(r.Context(), ticketID, userID, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStatus):
			response.Error(w, http.StatusBadRequest, "invalid ticket status: must be open, in_progress, or closed")
		case errors.Is(err, service.ErrInvalidStatusTransition):
			response.Error(w, http.StatusConflict, "invalid status transition")
		case errors.Is(err, service.ErrTicketNotFound):
			response.Error(w, http.StatusNotFound, "ticket not found")
		case errors.Is(err, service.ErrUnauthorized):
			response.Error(w, http.StatusUnauthorized, "unauthorized")
		default:
			response.Error(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.JSON(w, http.StatusOK, updated)
}
