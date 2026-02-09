package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// globalTestMutex protects access to global variables across all tests
// This prevents race conditions when tests run in parallel
var globalTestMutex sync.Mutex

// testContext holds test-specific database and JWT secret to avoid modifying global state
type testContext struct {
	db        *sql.DB
	jwtSecret []byte
}

// setupTestContext creates an isolated test environment with its own DB connection and JWT secret
func setupTestContext(t *testing.T) *testContext {
	// Use test database or main database for integration tests
	connStr := "host=" + getEnv("DB_HOST", "localhost") +
		" port=" + getEnv("DB_PORT", "5432") +
		" user=" + getEnv("DB_USER", "postgres") +
		" password=" + getEnv("DB_PASSWORD", "postgres") +
		" dbname=" + getEnv("DB_NAME", "devops_db") +
		" sslmode=disable"

	testDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := testDB.Ping(); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}

	return &testContext{
		db:        testDB,
		jwtSecret: []byte(getEnv("JWT_SECRET", "test-secret-key")),
	}
}

// withTestContext temporarily sets global variables for a single handler execution
// This prevents race conditions by using a package-level mutex to serialize access to global state across all tests
func (tc *testContext) withTestContext(fn func()) {
	globalTestMutex.Lock()
	defer globalTestMutex.Unlock()

	// Save original global state
	originalDB := db
	originalJWTSecret := jwtSecret

	// Set test-specific state
	db = tc.db
	jwtSecret = tc.jwtSecret

	// Execute the handler
	fn()

	// Restore original state
	db = originalDB
	jwtSecret = originalJWTSecret
}

// Close cleans up the test context
func (tc *testContext) Close() {
	if tc.db != nil {
		tc.db.Close()
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Clean up test data
func cleanupTestData(t *testing.T, tc *testContext, username string) {
	_, err := tc.db.Exec("DELETE FROM users WHERE username = $1", username)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test user %s: %v", username, err)
	}
}

// Test: User Login - Success
func TestLoginHandler_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	// Create test user
	testUsername := "test_login_user_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	testPassword := "testpassword123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash test password: %v", err)
	}

	_, err = tc.db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) ON CONFLICT (username) DO NOTHING",
		testUsername, string(hashedPassword), "user",
	)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer cleanupTestData(t, tc, testUsername)

	// Create request
	loginReq := LoginRequest{
		Username: testUsername,
		Password: testPassword,
	}
	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		loginHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var loginResp LoginResponse
	err = json.NewDecoder(rr.Body).Decode(&loginResp)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if loginResp.Token == "" {
		t.Error("Expected token in response, got empty string")
	}
}

// Test: User Login - Invalid Credentials
func TestLoginHandler_InvalidCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	// Create request with wrong password
	loginReq := LoginRequest{
		Username: "nonexistent_user",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		loginHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

// Test: JWT Validation - Valid Token
func TestValidateHandler_ValidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	// Generate valid token
	var token string
	var err error
	tc.withTestContext(func() {
		token, err = generateJWT(1, "testuser", "admin")
	})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create request
	validateReq := ValidateRequest{Token: token}
	body, _ := json.Marshal(validateReq)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		validateHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var validateResp ValidateResponse
	err = json.NewDecoder(rr.Body).Decode(&validateResp)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !validateResp.Valid {
		t.Error("Expected valid token, got invalid")
	}

	if validateResp.Role != "admin" {
		t.Errorf("Expected role 'admin', got '%s'", validateResp.Role)
	}
}

// Test: JWT Validation - Invalid Token
func TestValidateHandler_InvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	// Create request with invalid token
	validateReq := ValidateRequest{Token: "invalid.token.here"}
	body, _ := json.Marshal(validateReq)
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		validateHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var validateResp ValidateResponse
	err := json.NewDecoder(rr.Body).Decode(&validateResp)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if validateResp.Valid {
		t.Error("Expected invalid token, got valid")
	}
}

// Test: Get All Users - Success (Admin)
func TestGetUsersHandler_Success_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		getUsersHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var users []User
	err := json.NewDecoder(rr.Body).Decode(&users)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Success - users list returned (may be empty)
	t.Logf("Retrieved %d users from database", len(users))
}

// Test: Create User - Success
func TestCreateUserHandler_Success_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	testUsername := "test_create_user_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	defer cleanupTestData(t, tc, testUsername)

	// Create request
	createReq := CreateUserRequest{
		Username: testUsername,
		Password: "password123",
		Role:     "user",
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		createUserHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusCreated, rr.Body.String())
	}

	var user User
	err := json.NewDecoder(rr.Body).Decode(&user)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if user.Username != testUsername {
		t.Errorf("Expected username '%s', got '%s'", testUsername, user.Username)
	}

	if user.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", user.Role)
	}
}

// Test: Create User - Duplicate Username
func TestCreateUserHandler_DuplicateUsername(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	testUsername := "test_duplicate_user_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash test password: %v", err)
	}

	// Create initial user
	_, err = tc.db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) ON CONFLICT (username) DO NOTHING",
		testUsername, string(hashedPassword), "user",
	)
	if err != nil {
		t.Fatalf("Failed to create initial test user: %v", err)
	}
	defer cleanupTestData(t, tc, testUsername)

	// Try to create duplicate
	createReq := CreateUserRequest{
		Username: testUsername,
		Password: "password123",
		Role:     "user",
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		createUserHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusConflict {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusConflict)
	}
}

// Test: Update User - Success
func TestEditUserHandler_Success_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	// Create test user
	testUsername := "test_edit_user_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash test password: %v", err)
	}

	var userID int
	err = tc.db.QueryRow(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) ON CONFLICT (username) DO UPDATE SET role = $3 RETURNING id",
		testUsername, string(hashedPassword), "user",
	).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer cleanupTestData(t, tc, testUsername)

	// Create update request
	updateReq := UpdateUserRequest{Role: "premium"}
	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.Itoa(userID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		editUserHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusOK, rr.Body.String())
	}

	// Validate response body
	var updatedUser User
	err = json.NewDecoder(rr.Body).Decode(&updatedUser)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response contains correct data
	if updatedUser.ID != userID {
		t.Errorf("Expected user ID %d, got %d", userID, updatedUser.ID)
	}

	if updatedUser.Username != testUsername {
		t.Errorf("Expected username '%s', got '%s'", testUsername, updatedUser.Username)
	}

	if updatedUser.Role != "premium" {
		t.Errorf("Expected role 'premium', got '%s'", updatedUser.Role)
	}

	// Verify in database
	var role string
	err = tc.db.QueryRow("SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	if err != nil {
		t.Fatalf("Failed to query updated user: %v", err)
	}

	if role != "premium" {
		t.Errorf("Expected role 'premium', got '%s'", role)
	}
}

// Test: Delete User - Success
func TestDeleteUserHandler_Success_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Setup
	tc := setupTestContext(t)
	defer tc.Close()

	// Create test user
	testUsername := "test_delete_user_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash test password: %v", err)
	}

	var userID int
	err = tc.db.QueryRow(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id",
		testUsername, string(hashedPassword), "user",
	).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create delete request
	req := httptest.NewRequest(http.MethodDelete, "/users/"+strconv.Itoa(userID), nil)

	// Execute
	rr := httptest.NewRecorder()
	tc.withTestContext(func() {
		deleteUserHandler(rr, req)
	})

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusOK, rr.Body.String())
	}

	// Validate response body
	var response map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response contains expected fields
	if msg, ok := response["message"].(string); !ok || msg == "" {
		t.Error("Expected 'message' field in response")
	}

	if deletedID, ok := response["deleted_user_id"].(float64); !ok || int(deletedID) != userID {
		t.Errorf("Expected 'deleted_user_id' to be %d, got %v", userID, response["deleted_user_id"])
	}

	// Verify deletion in database
	var count int
	err = tc.db.QueryRow("SELECT COUNT(*) FROM users WHERE id = $1", userID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to verify deletion: %v", err)
	}

	if count != 0 {
		t.Error("User was not deleted from database")
	}
}

// Test: Database Connection
func TestDatabaseConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	err := tc.db.Ping()
	if err != nil {
		t.Fatalf("Database connection failed: %v", err)
	}
}
