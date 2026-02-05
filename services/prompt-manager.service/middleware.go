package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const (
	userIDKey   contextKey = "user_id"
	usernameKey contextKey = "username"
	roleKey     contextKey = "role"
)

var authServiceURL string

func initAuth() {
	authServiceURL = os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://auth:8001"
	}
	log.Printf("Auth service URL: %s", authServiceURL)
}

// authMiddleware validates JWT tokens via the auth service
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Call auth service to validate token
		validateReq := AuthValidateRequest{Token: token}
		reqBody, err := json.Marshal(validateReq)
		if err != nil {
			log.Printf("Error marshaling validate request: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		resp, err := http.Post(authServiceURL+"/validate", "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			log.Printf("Error calling auth service: %v", err)
			http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		var validateResp AuthValidateResponse
		if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
			log.Printf("Error decoding validate response: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !validateResp.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Extract user info from JWT claims
		userID, username := extractUserInfoFromToken(token)
		if userID == 0 {
			log.Printf("Could not extract user ID from token")
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		ctx = context.WithValue(ctx, usernameKey, username)
		ctx = context.WithValue(ctx, roleKey, validateResp.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// extractUserInfoFromToken parses JWT claims to get user_id and username
func extractUserInfoFromToken(token string) (int, string) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, ""
	}

	// Decode the payload (second part) using base64 URL decoding
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Printf("Error decoding token payload: %v", err)
		return 0, ""
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		log.Printf("Error parsing token claims: %v", err)
		return 0, ""
	}

	var userID int
	var username string

	// Extract user_id (added by enhanced auth service)
	if uid, ok := claims["user_id"].(float64); ok {
		userID = int(uid)
	}

	// Extract username
	if uname, ok := claims["username"].(string); ok {
		username = uname
	}

	return userID, username
}

// getUserID extracts user ID from request context
func getUserID(r *http.Request) int {
	if userID, ok := r.Context().Value(userIDKey).(int); ok {
		return userID
	}
	return 0
}

// getUsername extracts username from request context
func getUsername(r *http.Request) string {
	if username, ok := r.Context().Value(usernameKey).(string); ok {
		return username
	}
	return ""
}

// getUserRole extracts user role from request context
func getUserRole(r *http.Request) string {
	if role, ok := r.Context().Value(roleKey).(string); ok {
		return role
	}
	return ""
}
