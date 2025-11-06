package middleware

import (
	"net/http"
	"strings"

	"s3-test-app/internal/auth"
)

// AuthMiddleware validates tokens and extracts user information
func AuthMiddleware(tokenManager *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenString string

			// First, try to get token from cookie (for HTML page requests)
			if cookie, err := r.Cookie("auth_token"); err == nil {
				tokenString = cookie.Value
			} else {
				// Fall back to Authorization header (for API requests from JS)
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Extract token from "Bearer <token>" format
				parts := strings.Split(authHeader, " ")
				if len(parts) != 2 || parts[0] != "Bearer" {
					http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
					return
				}

				tokenString = parts[1]
			}

			// Validate token
			claims, err := tokenManager.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Add user to context
			user := claims.ToUser()
			*r = *r.WithContext(auth.SetUserInContext(r.Context(), user))

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole middleware checks if user has required role
func RequireRole(role auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if user.Role != auth.RoleAdmin && user.Role != role {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission middleware checks if user has required permission
func RequirePermission(perm auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.GetUserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !user.HasPermission(perm) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}