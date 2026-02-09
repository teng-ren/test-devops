package main

import (
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"os"
	"testing"
	"time"
)

// setupTestJWT initializes the JWT secret for testing
func setupTestJWT() {
	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-only")
	jwtSecret = []byte("test-secret-key-for-unit-tests-only")
}

// clean up TestJWT cleans up test environment
func cleanupTestJWT() {
	os.Unsetenv("JWT_SECRET")
	jwtSecret = nil
}

// TestGenerateJWT_Success verifies successful JWT token generation
func TestGenerateJWT_Success(t *testing.T) {
	setupTestJWT()
	defer cleanupTestJWT()

	tests := []struct {
		name     string
		username string
		role     string
	}{
		{
			name:     "Generate token for admin user",
			username: "admin",
			role:     "admin",
		},
		{
			name:     "Generate token for regular user",
			username: "john_doe",
			role:     "user",
		},
		{
			name:     "Generate token for premium user",
			username: "premium_user",
			role:     "premium",
		},
		{
			name:     "Generate token with special characters in username",
			username: "user@example.com",
			role:     "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := generateJWT(1, tt.username, tt.role)

			if err != nil {
				t.Errorf("generateJWT() error = %v", err)
				return
			}

			if token == "" {
				t.Error("generateJWT() returned empty token")
				return
			}

			// Verify token can be parsed
			parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
				return jwtSecret, nil
			})

			if err != nil {
				t.Errorf("Failed to parse generated token: %v", err)
				return
			}

			if !parsedToken.Valid {
				t.Error("Generated token is not valid")
				return
			}

			// Verify claims
			_, ok := parsedToken.Claims.(jwt.MapClaims)
			if !ok {
				t.Error("Failed to extract claims from token")
				return
			}
		})
	}
}

// TestGenerateJWT_WithoutSecret verifies behavior when JWT_SECRET is not set
func TestGenerateJWT_WithoutSecret(t *testing.T) {
	// Clear JWT secret
	jwtSecret = nil

	_, err := generateJWT(1, "testuser", "user")

	// Should return an error when secret is not set
	if err == nil {
		t.Error("Expected error when JWT secret is not set, got nil")
	}

	// Restore for other tests
	setupTestJWT()
}

// JWT Validation Tests

// TestValidateJWT_ValidToken verifies validation of a valid token
func TestValidateJWT_ValidToken(t *testing.T) {
	setupTestJWT()
	defer cleanupTestJWT()

	tests := []struct {
		name     string
		username string
		role     string
	}{
		{
			name:     "Validate admin token",
			username: "admin",
			role:     "admin",
		},
		{
			name:     "Validate user token",
			username: "john",
			role:     "user",
		},
		{
			name:     "Validate premium token",
			username: "premium",
			role:     "premium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate token
			token, err := generateJWT(1, tt.username, tt.role)
			if err != nil {
				t.Fatalf("Failed to generate token: %v", err)
			}

			valid, role := validateJWT(token)

			if !valid {
				t.Error("validateJWT() returned false for valid token")
			}

			if role != tt.role {
				t.Errorf("validateJWT() role = %v, want %v", role, tt.role)
			}
		})
	}
}

// TestValidateJWT_ExpiredToken verifies expired tokens are rejected
func TestValidateJWT_ExpiredToken(t *testing.T) {
	setupTestJWT()
	defer cleanupTestJWT()

	claims := jwt.MapClaims{
		"username": "testuser",
		"role":     "user",
		"exp":      time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
		"iat":      time.Now().Add(-time.Hour * 25).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)

	valid, role := validateJWT(tokenString)

	if valid {
		t.Error("validateJWT() should return false for expired token")
	}

	if role != "" {
		t.Error("validateJWT() should return empty role for expired token")
	}
}

// Password Hashing Tests

// TestCheckPassword_Valid verifies password verification with correct password
func TestCheckPassword_Valid(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "valid password case 1",
			password: "password",
		},
		{
			name:     "valid password case 2",
			password: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := bcrypt.GenerateFromPassword([]byte(tt.password), bcrypt.DefaultCost)
			if err != nil {
				t.Fatalf("Failed to hash password: %v", err)
			}

			result := checkPassword(string(hash), tt.password)

			if !result {
				t.Error("checkPassword() returned false for correct password")
			}
		})
	}
}

// TestCheckPassword_Invalid verifies password verification with wrong password
func TestCheckPassword_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		correctPass   string
		attemptedPass string
	}{
		{
			name:          "Wrong password",
			correctPass:   "correctpassword",
			attemptedPass: "wrongpassword",
		},
		{
			name:          "Case sensitivity",
			correctPass:   "Password123",
			attemptedPass: "password123",
		},
		{
			name:          "Extra character",
			correctPass:   "password",
			attemptedPass: "password1",
		},
		{
			name:          "Missing character",
			correctPass:   "password123",
			attemptedPass: "password12",
		},
		{
			name:          "Empty password attempt",
			correctPass:   "password",
			attemptedPass: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := bcrypt.GenerateFromPassword([]byte(tt.correctPass), bcrypt.DefaultCost)
			if err != nil {
				t.Fatalf("Failed to hash password: %v", err)
			}

			result := checkPassword(string(hash), tt.attemptedPass)

			if result {
				t.Error("checkPassword() returned true for incorrect password")
			}
		})
	}
}

// TestGetUserByUsername_Success verifies successful user retrieval
func TestGetUserByUsername_Success(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	username := "testuser"
	passwordHash := "hashedpassword123"
	role := "user"

	// Setup mock expectations
	rows := sqlmock.NewRows([]string{"id", "username", "password_hash", "role"}).
		AddRow(1, username, passwordHash, role)
	mock.ExpectQuery("SELECT id, username, password_hash, role FROM users WHERE username = \\$1").
		WithArgs(username).
		WillReturnRows(rows)

	user, err := getUserByUsername(username)

	if err != nil {
		t.Errorf("getUserByUsername() error = %v", err)
		return
	}

	if user == nil {
		t.Error("getUserByUsername() returned nil user")
		return
	}

	if user.Username != username {
		t.Errorf("Username = %v, want %v", user.Username, username)
	}

	if user.Role != role {
		t.Errorf("Role = %v, want %v", user.Role, role)
	}

	if user.PasswordHash != passwordHash {
		t.Errorf("PasswordHash = %v, want %v", user.PasswordHash, passwordHash)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestGetUserByUsername_NotFound verifies handling when user doesn't exist
func TestGetUserByUsername_NotFound(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	username := "nonexistentuser"

	// Setup mock to return no rows
	mock.ExpectQuery("SELECT id, username, password_hash, role FROM users WHERE username = \\$1").
		WithArgs(username).
		WillReturnError(sql.ErrNoRows)

	user, err := getUserByUsername(username)

	if err == nil {
		t.Error("Expected error for non-existent user, got nil")
	}

	if user != nil {
		t.Errorf("Expected nil user for non-existent user, got: %v", user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestGetUserByUsername_EmptyUsername verifies handling of empty username
func TestGetUserByUsername_EmptyUsername(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup mock to return no rows for empty username
	mock.ExpectQuery("SELECT id, username, password_hash, role FROM users WHERE username = \\$1").
		WithArgs("").
		WillReturnError(sql.ErrNoRows)

	user, err := getUserByUsername("")

	if err == nil {
		t.Error("Expected error for empty username, got nil")
	}

	if user != nil {
		t.Errorf("Expected nil user for empty username, got: %v", user)
	}
}
