package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
	"s3-test-app/internal/auth"
	"s3-test-app/internal/config"
	"s3-test-app/internal/db"
	"s3-test-app/internal/handler"
	mw "s3-test-app/internal/middleware"
	"s3-test-app/internal/service"
)

func main() {
	// Load configuration
	cfg := config.NewConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Initialize database
	database, err := db.New(cfg.Database.Path)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer database.Close()

	// Initialize S3 service
	s3Svc, err := service.NewS3Service(&cfg.S3, logger)
	if err != nil {
		logger.Fatal("Failed to initialize S3 service", zap.Error(err))
	}

	// Create token manager
	tokenManager := auth.NewTokenManager(cfg.Auth.Secret)

	// Create handlers
	h := handler.NewHandler(s3Svc, logger)
	authHandler := handler.NewAuthHandler(tokenManager, database, logger, cfg)
	adminHandler := handler.NewAdminHandler(database, logger)

	// Create router
	r := chi.NewRouter()

	// Middleware (all before routes)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Public Routes
	r.Get("/", h.GetIndex)
	r.Get("/login", handler.GetLogin)
	r.Get("/signup", handler.GetSignup)
	r.Get("/health", h.HealthCheck)

	// Auth Routes (public)
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/login", authHandler.LoginHandler)
		r.Post("/signup", authHandler.SignupHandler)
		r.Post("/logout", authHandler.LogoutHandler)
	})

	// Protected Routes (require authentication)
	r.Group(func(r chi.Router) {
		r.Use(mw.AuthMiddleware(tokenManager))
		r.Get("/dashboard", handler.GetDashboard)

		// API Routes (require authentication)
		r.Route("/api", func(r chi.Router) {
			r.Get("/files", h.ListFiles)
			r.Post("/upload", h.UploadFile)
			r.Get("/download", h.DownloadFile)
			r.Delete("/files", h.DeleteFile)
		})

		// Admin Routes (require admin role)
		r.Route("/api/admin", func(r chi.Router) {
			r.Use(mw.RequireRole(auth.RoleAdmin))
			r.Get("/users", adminHandler.GetUsers)
			r.Delete("/users/{id}", adminHandler.DeleteUser)
		})
	})

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	logger.Info("Starting server", zap.String("address", addr), zap.String("s3_endpoint", cfg.S3.Endpoint), zap.String("bucket", cfg.S3.Bucket))

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	<-sigChan
	logger.Info("Shutting down server...")
	if err := server.Close(); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}
}
