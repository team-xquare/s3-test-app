package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
	"s3-test-app/internal/auth"
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

// NewAuthHandler creates a new auth handler
func NewAuthHandler(tokenManager *auth.TokenManager, database *db.Database, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		tokenManager: tokenManager,
		database:     database,
		logger:       logger,
	}
}

// AuthHandler handles authentication
type AuthHandler struct {
	tokenManager *auth.TokenManager
	database     *db.Database
	logger       *zap.Logger
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