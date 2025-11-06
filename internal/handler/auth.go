package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"s3-test-app/internal/auth"
	"s3-test-app/internal/config"
	"s3-test-app/internal/db"
)

// User credentials for login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SignupRequest for user registration
type SignupRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	SignupKey string `json:"signup_key"`
}

type SignupResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Error   string `json:"error,omitempty"`
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(tokenManager *auth.TokenManager, database *db.Database, logger *zap.Logger, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		tokenManager: tokenManager,
		database:     database,
		logger:       logger,
		cfg:          cfg,
	}
}

// AuthHandler handles authentication
type AuthHandler struct {
	tokenManager *auth.TokenManager
	database     *db.Database
	logger       *zap.Logger
	cfg          *config.Config
}

// LoginHandler handles user login
func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Error:   "method not allowed",
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Error:   "invalid request",
		})
		return
	}

	// Get user from database
	dbUser, err := h.database.GetUserByUsername(req.Username)
	if err != nil {
		h.logger.Warn("login failed - user not found", zap.String("username", req.Username))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Error:   "invalid username or password",
		})
		return
	}

	// Verify password
	if !db.VerifyPassword(dbUser.Password, req.Password) {
		h.logger.Warn("login failed - invalid password", zap.String("username", req.Username))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Error:   "invalid username or password",
		})
		return
	}

	// Create user object
	user := &auth.User{
		ID:    dbUser.ID,
		Name:  dbUser.Username,
		Email: dbUser.Email,
		Role:  dbUser.Role,
	}

	// Generate token
	token, err := h.tokenManager.GenerateToken(user, 24*time.Hour)
	if err != nil {
		h.logger.Error("failed to generate token", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Error:   "failed to generate token",
		})
		return
	}

	h.logger.Info("user logged in", zap.String("username", req.Username), zap.String("role", string(user.Role)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Success: true,
		Token:   token,
	})
}

// SignupHandler handles user registration
func (h *AuthHandler) SignupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "method not allowed",
		})
		return
	}

	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "invalid request",
		})
		return
	}

	// Validate signup key
	if req.SignupKey != h.cfg.Auth.SignupKey {
		h.logger.Warn("signup failed - invalid signup key", zap.String("username", req.Username))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "invalid signup key",
		})
		return
	}

	// Validate input
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	if req.Username == "" || req.Email == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "username, email and password are required",
		})
		return
	}

	if len(req.Password) < 6 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "password must be at least 6 characters",
		})
		return
	}

	// Check if user already exists
	if _, err := h.database.GetUserByUsername(req.Username); err == nil {
		h.logger.Warn("signup failed - user already exists", zap.String("username", req.Username))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "username already exists",
		})
		return
	}

	// Check if email already exists
	if _, err := h.database.GetUserByEmail(req.Email); err == nil {
		h.logger.Warn("signup failed - email already exists", zap.String("email", req.Email))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "email already exists",
		})
		return
	}

	// Generate user ID
	userID := "user_" + time.Now().Format("20060102150405")

	// Create user
	if err := h.database.CreateUser(userID, req.Username, req.Email, req.Password, auth.RoleUploader); err != nil {
		h.logger.Error("failed to create user", zap.String("username", req.Username), zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "failed to create user",
		})
		return
	}

	// Create user object
	user := &auth.User{
		ID:    userID,
		Name:  req.Username,
		Email: req.Email,
		Role:  auth.RoleUploader,
	}

	// Generate token
	token, err := h.tokenManager.GenerateToken(user, 24*time.Hour)
	if err != nil {
		h.logger.Error("failed to generate token", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SignupResponse{
			Success: false,
			Error:   "failed to generate token",
		})
		return
	}

	h.logger.Info("user registered", zap.String("username", req.Username), zap.String("email", req.Email))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SignupResponse{
		Success: true,
		Token:   token,
	})
}