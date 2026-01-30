package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract Bearer token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" || tokenString == authHeader {
			http.Error(w, "Bearer token required", http.StatusUnauthorized)
			return
		}

		// Create validation request
		validateReq := ValidateRequest{Token: tokenString}
		jsonData, err := json.Marshal(validateReq)
		if err != nil {
			http.Error(w, "Failed to process token", http.StatusInternalServerError)
			log.Printf("Error marshaling validation request: %v", err)
			return
		}

		// Get auth service URL from environment
		authURL := os.Getenv("AUTH_SERVICE_URL")
		if authURL == "" {
			authURL = "http://auth:8001" // fallback
		}

		// Call auth service validation endpoint
		resp, err := http.Post(authURL+"/validate", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
			log.Printf("Error calling auth service: %v", err)
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			log.Printf("Auth service returned status: %d", resp.StatusCode)
			return
		}

		// Parse validation response
		var validateResp ValidateResponse
		if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
			http.Error(w, "Failed to validate token", http.StatusInternalServerError)
			log.Printf("Error decoding validation response: %v", err)
			return
		}

		// Check if token is valid and user has admin role
		if !validateResp.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		if validateResp.Role != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		// Token is valid and user is admin, proceed to next handler
		next.ServeHTTP(w, r)
	}
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, username, role, created_at FROM users ORDER BY id")
	if err != nil {
		http.Error(w, "Failed to query users", http.StatusInternalServerError)
		log.Printf("Error querying users: %v", err)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt); err != nil {
			http.Error(w, "Failed to scan user", http.StatusInternalServerError)
			log.Printf("Error scanning user: %v", err)
			return
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterating users", http.StatusInternalServerError)
		log.Printf("Error iterating users: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role (supports admin, user, premium)
	if req.Role != "admin" && req.Role != "user" && req.Role != "premium" {
		http.Error(w, "Invalid role. Must be admin, user, or premium", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", req.Username).Scan(&exists)
	if err != nil {
		http.Error(w, "Failed to check existing user", http.StatusInternalServerError)
		log.Printf("Error checking existing user: %v", err)
		return
	}

	if exists {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		log.Printf("Error hashing password: %v", err)
		return
	}

	// Create user
	var userID int
	err = db.QueryRow(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id",
		req.Username, string(hashedPassword), req.Role,
	).Scan(&userID)

	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		log.Printf("Error creating user: %v", err)
		return
	}

	// Return created user without password
	user := User{
		ID:        userID,
		Username:  req.Username,
		Role:      req.Role,
		CreatedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func editUserHandler(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/users/")
	userID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role (supports admin, user, premium)
	if req.Role != "admin" && req.Role != "user" && req.Role != "premium" {
		http.Error(w, "Invalid role. Must be admin, user, or premium", http.StatusBadRequest)
		return
	}

	// Get target user info
	var targetUsername string
	var currentRole string

	err = db.QueryRow("SELECT username, role FROM users WHERE id = $1", userID).Scan(&targetUsername, &currentRole)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get user info", http.StatusInternalServerError)
			log.Printf("Error getting user info: %v", err)
		}
		return
	}

	// Update user role
	result, err := db.Exec("UPDATE users SET role = $1 WHERE id = $2", req.Role, userID)
	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		log.Printf("Error updating user: %v", err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Return updated user
	updatedUser := User{
		ID:        userID,
		Username:  targetUsername,
		Role:      req.Role,
		CreatedAt: time.Now(), // Will be populated from DB in real implementation
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedUser)
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/users/")
	userID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check if user exists before deletion
	var username string
	err = db.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&username)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to check user", http.StatusInternalServerError)
			log.Printf("Error checking user: %v", err)
		}
		return
	}

	// Delete user
	result, err := db.Exec("DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		log.Printf("Error deleting user: %v", err)
		return
	}

	// Verify deletion was successful
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":         "User deleted successfully",
		"deleted_user_id": userID,
	})
}
