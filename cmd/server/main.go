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

	// Initialize demo users
	if err := initializeDemoUsers(database, logger); err != nil {
		logger.Fatal("Failed to initialize demo users", zap.Error(err))
	}

	// Initialize S3 service
	s3Svc, err := service.NewS3Service(&cfg.S3, logger)
	if err != nil {
		logger.Fatal("Failed to initialize S3 service", zap.Error(err))
	}

	// Create token manager
	tokenManager := auth.NewTokenManager(cfg.Auth.Secret)

	// Create handlers
	h := handler.NewHandler(s3Svc, logger)
	authHandler := handler.NewAuthHandler(tokenManager, database, logger)
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
	r.Get("/health", h.HealthCheck)

	// Auth Routes (public)
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/login", authHandler.LoginHandler)
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

// initializeDemoUsers creates demo users if they don't exist
func initializeDemoUsers(database *db.Database, logger *zap.Logger) error {
	demoUsers := []struct {
		id       string
		username string
		email    string
		password string
		role     auth.Role
	}{
		{
			id:       "admin",
			username: "admin",
			email:    "admin@example.com",
			password: "admin123",
			role:     auth.RoleAdmin,
		},
		{
			id:       "uploader",
			username: "uploader",
			email:    "uploader@example.com",
			password: "uploader123",
			role:     auth.RoleUploader,
		},
		{
			id:       "viewer",
			username: "viewer",
			email:    "viewer@example.com",
			password: "viewer123",
			role:     auth.RoleViewer,
		},
	}

	for _, user := range demoUsers {
		// Check if user already exists
		_, err := database.GetUserByID(user.id)
		if err == nil {
			// User already exists, skip
			logger.Info("Demo user already exists", zap.String("username", user.username))
			continue
		}

		// Create user
		if err := database.CreateUser(user.id, user.username, user.email, user.password, user.role); err != nil {
			logger.Warn("Failed to create demo user", zap.String("username", user.username), zap.Error(err))
			// Don't fail on duplicate users, continue
			continue
		}

		logger.Info("Created demo user", zap.String("username", user.username), zap.String("role", string(user.role)))
	}

	return nil
}