package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"s3-test-app/internal/auth"
	"s3-test-app/internal/service"
	"s3-test-app/templates"
)

// Handler holds HTTP handlers
type Handler struct {
	s3Service *service.S3Service
	logger    *zap.Logger
}

// NewHandler creates a new Handler
func NewHandler(s3Service *service.S3Service, logger *zap.Logger) *Handler {
	return &Handler{
		s3Service: s3Service,
		logger:    logger,
	}
}

// Response is a generic API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// HealthCheck handles the health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GetLogin handles the login page
func GetLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.Login().Render(r.Context(), w)
}

// GetDashboard handles the dashboard page
func GetDashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.Dashboard(user.Name, string(user.Role)).Render(r.Context(), w)
}

// GetIndex handles the root endpoint (redirect to dashboard)
func (h *Handler) GetIndex(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// ListFiles handles the list files endpoint
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	files, err := h.s3Service.ListFiles(ctx)
	if err != nil {
		h.logger.Error("failed to list files", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"files": files,
			"count": len(files),
		},
	})
}

// UploadFile handles the file upload endpoint
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "method not allowed",
		})
		return
	}

	ctx := r.Context()
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.logger.Error("failed to parse multipart form", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "failed to parse form",
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.Error("failed to get form file", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "file not provided",
		})
		return
	}
	defer file.Close()

	// Read file content
	buf := make([]byte, header.Size)
	if _, err := file.Read(buf); err != nil {
		h.logger.Error("failed to read file", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "failed to read file",
		})
		return
	}

	// Create unique key
	key := fmt.Sprintf("%d-%s", time.Now().Unix(), header.Filename)

	// Upload to S3
	if err := h.s3Service.UploadFile(ctx, key, buf); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"key":      key,
			"filename": header.Filename,
			"size":     header.Size,
		},
	})
}

// DownloadFile handles the file download endpoint
func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.URL.Query().Get("key")

	if key == "" {
		http.Error(w, "key parameter required", http.StatusBadRequest)
		return
	}

	data, err := h.s3Service.GetFile(ctx, key)
	if err != nil {
		h.logger.Error("failed to download file", zap.String("key", key), zap.Error(err))
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", key))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

// DeleteFile handles the file delete endpoint
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "method not allowed",
		})
		return
	}

	ctx := r.Context()
	key := r.URL.Query().Get("key")

	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "key parameter required",
		})
		return
	}

	if err := h.s3Service.DeleteFile(ctx, key); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"message": "file deleted successfully",
		},
	})
}