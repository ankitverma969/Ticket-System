package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

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
