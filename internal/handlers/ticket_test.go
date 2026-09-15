package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/auth"
	"ticket-system/internal/handlers"
	appMiddleware "ticket-system/internal/middleware"
	"ticket-system/internal/models"
	"ticket-system/internal/repository"
	"ticket-system/internal/service"
)

// mockTicketRepo provides an in-memory ticket repository for HTTP handler unit tests.
type mockTicketRepo struct {
	tickets []*models.Ticket
}

func newMockTicketRepo() *mockTicketRepo {
	return &mockTicketRepo{
		tickets: make([]*models.Ticket, 0),
	}
}

func (m *mockTicketRepo) CreateTicket(ctx context.Context, ticket *models.Ticket) error {
	if ticket.ID.IsZero() {
		ticket.ID = bson.NewObjectID()
	}
	now := time.Now().UTC()
	ticket.CreatedAt = now
	ticket.UpdatedAt = now
	m.tickets = append(m.tickets, ticket)
	return nil
}

func (m *mockTicketRepo) GetTicketsByUserID(ctx context.Context, userID bson.ObjectID) ([]*models.Ticket, error) {
	result := make([]*models.Ticket, 0)
	// Iterate backwards so newest tickets come first
	for i := len(m.tickets) - 1; i >= 0; i-- {
		t := m.tickets[i]
		if t.UserID == userID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTicketRepo) GetTicketByIDAndUserID(ctx context.Context, ticketID, userID bson.ObjectID) (*models.Ticket, error) {
	for _, t := range m.tickets {
		if t.ID == ticketID && t.UserID == userID {
			return t, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockTicketRepo) UpdateTicketStatus(ctx context.Context, ticketID, userID bson.ObjectID, status models.TicketStatus) error {
	for _, t := range m.tickets {
		if t.ID == ticketID && t.UserID == userID {
			t.Status = status
			t.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return repository.ErrNotFound
}

func setupTicketTestRouter() (http.Handler, *auth.TokenManager, bson.ObjectID, bson.ObjectID) {
	secret := "ticket-test-secret-key-12345678"
	tm, _ := auth.NewTokenManager(secret, 1*time.Hour)

	repo := newMockTicketRepo()
	svc := service.NewTicketService(repo)
	handler := handlers.NewTicketHandler(svc)

	r := chi.NewRouter()
	r.Group(func(protected chi.Router) {
		protected.Use(appMiddleware.Auth(tm))
		protected.Post("/tickets", handler.Create)
		protected.Get("/tickets", handler.List)
		protected.Get("/tickets/{id}", handler.GetByID)
	})

	userA := bson.NewObjectID()
	userB := bson.NewObjectID()

	return r, tm, userA, userB
}

func TestTicketEndpoints_SecurityAndOwnership(t *testing.T) {
	router, tm, userA, userB := setupTicketTestRouter()

	tokenA, _ := tm.GenerateToken(userA)
	tokenB, _ := tm.GenerateToken(userB)

	// 1. POST /tickets without JWT -> 401
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{"title":"T1","description":"D1"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing JWT on POST /tickets, got %d", rr.Code)
		}
	}

	// 2. GET /tickets without JWT -> 401
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing JWT on GET /tickets, got %d", rr.Code)
		}
	}

	// 3. POST /tickets with invalid JWT -> 401
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{"title":"T1","description":"D1"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for invalid JWT, got %d", rr.Code)
		}
	}

	// 4. Validation: Missing title -> 400
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{"title":"","description":"valid desc"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty title, got %d", rr.Code)
		}
	}

	// 5. Validation: Whitespace-only description -> 400
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{"title":"valid title","description":"   "}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for whitespace-only description, got %d", rr.Code)
		}
	}

	// 6. Validation: Malformed JSON -> 400
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{not valid json}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for malformed JSON, got %d", rr.Code)
		}
	}

	// 7. Security: Client cannot control ownership by providing user_id in body
	// User A attempts to set user_id to User B's ID
	{
		maliciousPayload := map[string]interface{}{
			"user_id":     userB.Hex(),
			"title":       "Malicious ownership attempt",
			"description": "Attempting to create ticket for user B",
			"status":      "closed", // also attempting to bypass default status
		}
		bodyBytes, _ := json.Marshal(maliciousPayload)
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rr.Code, rr.Body.String())
		}

		var createdTicket service.TicketOutput
		_ = json.Unmarshal(rr.Body.Bytes(), &createdTicket)

		// The ticket MUST belong to User A, NOT User B
		if createdTicket.UserID != userA.Hex() {
			t.Errorf("SECURITY VULNERABILITY: Client controlled ownership! Expected user_id %s, got %s", userA.Hex(), createdTicket.UserID)
		}

		// The status MUST be "open", NOT "closed"
		if createdTicket.Status != models.StatusOpen {
			t.Errorf("SECURITY VULNERABILITY: Client controlled status! Expected 'open', got %s", createdTicket.Status)
		}
	}

	// 8. User B creates a ticket
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{"title":"User B Issue","description":"Problem for user B"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenB)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for user B, got %d: %s", rr.Code, rr.Body.String())
		}
	}

	// 9. Critical Cross-User Test: User A queries GET /tickets -> must ONLY see Ticket A
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for user A list, got %d: %s", rr.Code, rr.Body.String())
		}

		var listA []service.TicketOutput
		_ = json.Unmarshal(rr.Body.Bytes(), &listA)

		if len(listA) != 1 {
			t.Fatalf("expected 1 ticket for user A, got %d", len(listA))
		}
		if listA[0].UserID != userA.Hex() {
			t.Errorf("expected ticket to belong to user A, got %s", listA[0].UserID)
		}
		if listA[0].Title != "Malicious ownership attempt" {
			t.Errorf("unexpected ticket title for user A: %s", listA[0].Title)
		}
	}

	// 10. Critical Cross-User Test: User B queries GET /tickets -> must ONLY see Ticket B
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets", nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for user B list, got %d: %s", rr.Code, rr.Body.String())
		}

		var listB []service.TicketOutput
		_ = json.Unmarshal(rr.Body.Bytes(), &listB)

		if len(listB) != 1 {
			t.Fatalf("expected 1 ticket for user B, got %d", len(listB))
		}
		if listB[0].UserID != userB.Hex() {
			t.Errorf("expected ticket to belong to user B, got %s", listB[0].UserID)
		}
		if listB[0].Title != "User B Issue" {
			t.Errorf("unexpected ticket title for user B: %s", listB[0].Title)
		}
	}

	// 11. Empty Result Behavior: New User C with no tickets gets empty array `[]`
	{
		userC := bson.NewObjectID()
		tokenC, _ := tm.GenerateToken(userC)

		req, _ := http.NewRequest(http.MethodGet, "/tickets", nil)
		req.Header.Set("Authorization", "Bearer "+tokenC)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for user C, got %d", rr.Code)
		}

		bodyStr := rr.Body.String()
		if bytes.Equal(rr.Body.Bytes(), []byte("null\n")) || bytes.Equal(rr.Body.Bytes(), []byte("null")) {
			t.Errorf("expected empty JSON array '[]', got 'null'")
		}

		var listC []service.TicketOutput
		_ = json.Unmarshal(rr.Body.Bytes(), &listC)
		if len(listC) != 0 {
			t.Errorf("expected 0 tickets for user C, got %d (raw: %s)", len(listC), bodyStr)
		}
	}
}

func TestGetTicketByID_SecurityAndOwnership(t *testing.T) {
	router, tm, userA, userB := setupTicketTestRouter()

	tokenA, _ := tm.GenerateToken(userA)
	tokenB, _ := tm.GenerateToken(userB)

	// 1. User A creates Ticket A
	var ticketA service.TicketOutput
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{"title":"User A Ticket","description":"Description for ticket A"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 for ticket A creation, got %d: %s", rr.Code, rr.Body.String())
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &ticketA)
	}

	// 2. User B creates Ticket B
	var ticketB service.TicketOutput
	{
		req, _ := http.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(`{"title":"User B Ticket","description":"Description for ticket B"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenB)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 for ticket B creation, got %d: %s", rr.Code, rr.Body.String())
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &ticketB)
	}

	// 3. User A can retrieve User A's Ticket A -> 200 OK
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets/"+ticketA.ID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for User A fetching Ticket A, got %d: %s", rr.Code, rr.Body.String())
		}

		var fetched service.TicketOutput
		_ = json.Unmarshal(rr.Body.Bytes(), &fetched)
		if fetched.ID != ticketA.ID || fetched.Title != "User A Ticket" {
			t.Errorf("expected ticket A data, got %+v", fetched)
		}
	}

	// 4. User B can retrieve User B's Ticket B -> 200 OK
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets/"+ticketB.ID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for User B fetching Ticket B, got %d: %s", rr.Code, rr.Body.String())
		}

		var fetched service.TicketOutput
		_ = json.Unmarshal(rr.Body.Bytes(), &fetched)
		if fetched.ID != ticketB.ID || fetched.Title != "User B Ticket" {
			t.Errorf("expected ticket B data, got %+v", fetched)
		}
	}

	// 5. CRITICAL: User A attempts to retrieve User B's Ticket B -> 404 Not Found
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets/"+ticketB.ID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("SECURITY FAW: expected 404 for User A accessing User B's ticket, got %d: %s", rr.Code, rr.Body.String())
		}

		// Verify zero data leakage: must not contain title, description, or user info
		bodyStr := rr.Body.String()
		if bytes.Contains(rr.Body.Bytes(), []byte("User B Ticket")) || bytes.Contains(rr.Body.Bytes(), []byte("Description for ticket B")) {
			t.Errorf("DATA LEAKAGE: Response leaked User B's ticket details: %s", bodyStr)
		}

		var errResp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &errResp)
		if errResp["error"] != "ticket not found" {
			t.Errorf("expected generic error 'ticket not found', got %q", errResp["error"])
		}
	}

	// 6. CRITICAL: User B attempts to retrieve User A's Ticket A -> 404 Not Found
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets/"+ticketA.ID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("SECURITY FAW: expected 404 for User B accessing User A's ticket, got %d: %s", rr.Code, rr.Body.String())
		}

		bodyStr := rr.Body.String()
		if bytes.Contains(rr.Body.Bytes(), []byte("User A Ticket")) || bytes.Contains(rr.Body.Bytes(), []byte("Description for ticket A")) {
			t.Errorf("DATA LEAKAGE: Response leaked User A's ticket details: %s", bodyStr)
		}
	}

	// 7. Nonexistent valid ObjectID -> 404 Not Found
	{
		nonExistentID := bson.NewObjectID().Hex()
		req, _ := http.NewRequest(http.MethodGet, "/tickets/"+nonExistentID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 for nonexistent ticket, got %d: %s", rr.Code, rr.Body.String())
		}
	}

	// 8. Malformed/Invalid ticket ID -> 400 Bad Request
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets/not-a-valid-object-id", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for malformed ticket ID, got %d: %s", rr.Code, rr.Body.String())
		}

		var errResp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &errResp)
		if errResp["error"] != "invalid ticket id" {
			t.Errorf("expected error 'invalid ticket id', got %q", errResp["error"])
		}
	}

	// 9. Missing Authorization header -> 401 Unauthorized
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets/"+ticketA.ID, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for unauthenticated request, got %d", rr.Code)
		}
	}

	// 10. Invalid JWT -> 401 Unauthorized
	{
		req, _ := http.NewRequest(http.MethodGet, "/tickets/"+ticketA.ID, nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for invalid token, got %d", rr.Code)
		}
	}
}
