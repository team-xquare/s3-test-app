package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TokenManager handles token operations using simple HMAC-based tokens
type TokenManager struct {
	secret string
}

// Claims represents token claims
type Claims struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	ExpiresAt time.Time `json:"exp"`
	IssuedAt  time.Time `json:"iat"`
}

// NewTokenManager creates a new token manager
func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{
		secret: secret,
	}
}

// GenerateToken generates a token for a user
func (m *TokenManager) GenerateToken(user *User, expirationTime time.Duration) (string, error) {
	claims := &Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		ExpiresAt: time.Now().Add(expirationTime),
		IssuedAt:  time.Now(),
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}

	// Create signature
	h := hmac.New(sha256.New, []byte(m.secret))
	h.Write(claimsJSON)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Encode claims
	encodedClaims := base64.StdEncoding.EncodeToString(claimsJSON)

	// Return token in format: claims.signature
	return encodedClaims + "." + signature, nil
}

// ValidateToken validates a token and returns claims
func (m *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	encodedClaims := parts[0]
	signature := parts[1]

	// Decode claims
	claimsJSON, err := base64.StdEncoding.DecodeString(encodedClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	// Verify signature
	h := hmac.New(sha256.New, []byte(m.secret))
	h.Write(claimsJSON)
	expectedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Parse claims
	claims := &Claims{}
	if err := json.Unmarshal(claimsJSON, claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Check expiration
	if time.Now().After(claims.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// ToUser converts claims to user
func (c *Claims) ToUser() *User {
	return &User{
		ID:    c.UserID,
		Name:  c.Name,
		Email: c.Email,
		Role:  c.Role,
	}
}