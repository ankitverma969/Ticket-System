package repository_test

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"ticket-system/internal/config"
	"ticket-system/internal/database"
	"ticket-system/internal/models"
	"ticket-system/internal/repository"
)

// setupTestDB connects to MongoDB if available, returning a cleanup function.
// If MongoDB is unreachable, it skips the integration tests gracefully.
func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	cfg := config.Load()
	// Use a dedicated isolated test database
	cfg.MongoDBDatabase = "ticket_system_test_" + bson.NewObjectID().Hex()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping MongoDB integration test (MongoDB is not available at %s: %v)", cfg.MongoDBURI, err)
		return nil, nil
	}

	if err := db.CreateIndexes(context.Background()); err != nil {
		_ = db.Database.Drop(context.Background())
		_ = db.Disconnect(context.Background())
		t.Fatalf("failed to create indexes in test db: %v", err)
	}

	cleanup := func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = db.Database.Drop(dropCtx)
		_ = db.Disconnect(dropCtx)
	}

	return db, cleanup
}

func TestMongoUserRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, cleanup := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanup()

	repo := repository.NewMongoUserRepository(db)
	ctx := context.Background()

	// 1. Create a user
	user := &models.User{
		Email:        "TestUser@Example.com",
		PasswordHash: "hashedsecret",
	}

	err := repo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("expected CreateUser to succeed, got: %v", err)
	}

	if user.ID.IsZero() {
		t.Errorf("expected user ID to be populated, got zero")
	}

	// 2. Get user by email (case normalization test)
	byEmail, err := repo.GetUserByEmail(ctx, "testuser@example.com")
	if err != nil {
		t.Fatalf("expected GetUserByEmail to find user, got: %v", err)
	}
	if byEmail.Email != "testuser@example.com" {
		t.Errorf("expected normalized email 'testuser@example.com', got %q", byEmail.Email)
	}

	// 3. Duplicate email test
	dupUser := &models.User{
		Email:        "testuser@example.com",
		PasswordHash: "anotherhash",
	}
	err = repo.CreateUser(ctx, dupUser)
	if err != repository.ErrDuplicate {
		t.Errorf("expected ErrDuplicate for existing email, got: %v", err)
	}

	// 4. Get user by ID
	byID, err := repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("expected GetUserByID to find user, got: %v", err)
	}
	if byID.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID.Hex(), byID.ID.Hex())
	}

	// 5. Not found check
	_, err = repo.GetUserByID(ctx, bson.NewObjectID())
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for random ID, got: %v", err)
	}
}

func TestMongoTicketRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, cleanup := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanup()

	repo := repository.NewMongoTicketRepository(db)
	ctx := context.Background()

	user1ID := bson.NewObjectID()
	user2ID := bson.NewObjectID()

	// 1. Create tickets for user 1
	ticket1 := &models.Ticket{
		UserID:      user1ID,
		Title:       "User 1 Issue",
		Description: "Something is broken",
		Status:      models.StatusOpen,
	}
	if err := repo.CreateTicket(ctx, ticket1); err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// 2. Create ticket for user 2
	ticket2 := &models.Ticket{
		UserID:      user2ID,
		Title:       "User 2 Issue",
		Description: "Another issue",
		Status:      models.StatusOpen,
	}
	if err := repo.CreateTicket(ctx, ticket2); err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// 3. Ownership-safe retrieval: User 1 queries their ticket -> succeeds
	found, err := repo.GetTicketByIDAndUserID(ctx, ticket1.ID, user1ID)
	if err != nil {
		t.Fatalf("expected user 1 to find their ticket, got: %v", err)
	}
	if found.Title != ticket1.Title {
		t.Errorf("expected title %q, got %q", ticket1.Title, found.Title)
	}

	// 4. Ownership enforcement: User 2 tries to query User 1's ticket -> ErrNotFound
	_, err = repo.GetTicketByIDAndUserID(ctx, ticket1.ID, user2ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound when user2 queries user1's ticket, got: %v", err)
	}

	// 5. Query user 1's list of tickets
	user1Tickets, err := repo.GetTicketsByUserID(ctx, user1ID)
	if err != nil {
		t.Fatalf("failed to get user 1 tickets: %v", err)
	}
	if len(user1Tickets) != 1 {
		t.Errorf("expected 1 ticket for user 1, got %d", len(user1Tickets))
	}

	// 6. Update ticket status
	err = repo.UpdateTicketStatus(ctx, ticket1.ID, user1ID, models.StatusInProgress)
	if err != nil {
		t.Fatalf("failed to update ticket status: %v", err)
	}

	updated, err := repo.GetTicketByIDAndUserID(ctx, ticket1.ID, user1ID)
	if err != nil {
		t.Fatalf("failed to fetch updated ticket: %v", err)
	}
	if updated.Status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", updated.Status)
	}

	// 7. Ownership enforcement on update: User 2 cannot update User 1's ticket
	err = repo.UpdateTicketStatus(ctx, ticket1.ID, user2ID, models.StatusClosed)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound when unauthorized user attempts status update, got: %v", err)
	}
}
