package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"ticket-system/internal/auth"
	"ticket-system/internal/config"
	"ticket-system/internal/database"
	"ticket-system/internal/handlers"
	appMiddleware "ticket-system/internal/middleware"
	"ticket-system/internal/repository"
	"ticket-system/internal/response"
	"ticket-system/internal/service"
)

func main() {
	cfg := config.Load()

	// Initialize MongoDB connection context
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()

	mongoDB, err := database.Connect(initCtx, cfg)
	if err != nil {
		log.Printf("Warning: Could not connect to MongoDB (%v). Server will still start and /health will work, but auth APIs will require MongoDB.", err)
	} else {
		log.Println("Successfully connected to MongoDB.")
		if err := mongoDB.CreateIndexes(initCtx); err != nil {
			log.Printf("Warning: Failed to create MongoDB indexes: %v", err)
		}
		defer func() {
			disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mongoDB.Disconnect(disconnectCtx)
		}()
	}

	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	// Custom NotFound and MethodNotAllowed handlers returning clean JSON errors
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, http.StatusNotFound, "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	// Health check endpoint (always public and independent of DB)
	r.Get("/health", handlers.Health)

	// Authentication routes
	if mongoDB != nil {
		tokenManager, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiration)
		if err != nil {
			log.Fatalf("Fatal: Failed to initialize TokenManager: %v", err)
		}

		userRepo := repository.NewMongoUserRepository(mongoDB)
		authSvc := service.NewAuthService(userRepo, tokenManager)
		authHandler := handlers.NewAuthHandler(authSvc)

		ticketRepo := repository.NewMongoTicketRepository(mongoDB)
		ticketSvc := service.NewTicketService(ticketRepo)
		ticketHandler := handlers.NewTicketHandler(ticketSvc)

		// Public authentication routes
		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)

		// Protected ticket routes
		r.Group(func(protected chi.Router) {
			protected.Use(appMiddleware.Auth(tokenManager))
			protected.Post("/tickets", ticketHandler.Create)
			protected.Get("/tickets", ticketHandler.List)
			protected.Get("/tickets/{id}", ticketHandler.GetByID)
			protected.Patch("/tickets/{id}/status", ticketHandler.UpdateStatus)
		})
	} else {
		// If MongoDB is not connected, register stubs that return 503 Service Unavailable
		r.Post("/auth/register", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
		})
		r.Post("/auth/login", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
		})
	}

	server := &http.Server{
		Addr:         "0.0.0.0:" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run context for graceful shutdown
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		shutdownCtx, cancel := context.WithTimeout(serverCtx, 15*time.Second)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()
			if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
				log.Println("Graceful shutdown timed out.. forcing exit.")
			}
		}()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during server shutdown: %v\n", err)
		}
		serverStopCtx()
	}()

	log.Printf("Starting Ticket System server on port %s...\n", cfg.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server failed to start: %v\n", err)
	}

	<-serverCtx.Done()
	log.Println("Server gracefully stopped.")
}
