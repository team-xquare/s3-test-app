package handler

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
	"s3-test-app/internal/auth"
	"s3-test-app/internal/db"
)

// AdminHandler handles admin operations
type AdminHandler struct {
	database *db.Database
	logger   *zap.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(database *db.Database, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		database: database,
		logger:   logger,
	}
}

// GetUsers returns all users (admin only)
func (h *AdminHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil || user.Role != auth.RoleAdmin {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "unauthorized",
		})
		return
	}

	// Get users from database
	dbUsers, err := h.database.GetAllUsers()
	if err != nil {
		h.logger.Error("failed to get users", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "failed to retrieve users",
		})
		return
	}

	// Convert database users to response format (exclude password hash)
	users := make([]map[string]interface{}, len(dbUsers))
	for i, dbUser := range dbUsers {
		users[i] = map[string]interface{}{
			"id":       dbUser.ID,
			"username": dbUser.Username,
			"email":    dbUser.Email,
			"role":     string(dbUser.Role),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"users": users,
		},
	})
}

// DeleteUser deletes a user (admin only)
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil || user.Role != auth.RoleAdmin {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "unauthorized",
		})
		return
	}

	if r.Method != http.MethodDelete {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "method not allowed",
		})
		return
	}

	userId := r.URL.Path[len("/api/admin/users/"):]

	// Prevent deleting self
	if userId == user.ID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "cannot delete yourself",
		})
		return
	}

	// Delete user from database
	if err := h.database.DeleteUser(userId); err != nil {
		h.logger.Warn("failed to delete user", zap.String("user_id", userId), zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}

	h.logger.Info("user deleted", zap.String("admin", user.ID), zap.String("deleted_user", userId))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"message": "user deleted",
		},
	})
}