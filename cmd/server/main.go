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
	"github.com/go-chi/chi/v5/middleware"

	"ticket-system/internal/auth"
	"ticket-system/internal/config"
	"ticket-system/internal/database"
	"ticket-system/internal/handlers"
	"ticket-system/internal/repository"
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
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint (always independent of DB)
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

		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)
	} else {
		// If MongoDB is not connected, register stub that returns 503 Service Unavailable
		r.Post("/auth/register", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
		})
		r.Post("/auth/login", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
		})
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
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
