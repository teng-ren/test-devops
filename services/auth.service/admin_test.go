package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// setupAdminTestDB initializes mock database for admin testing
func setupAdminTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}

	// Replace global db with mock
	originalDB := db
	db = mockDB

	// Set JWT secret and auth service URL
	os.Setenv("JWT_SECRET", "test-secret-key")
	os.Setenv("AUTH_SERVICE_URL", "http://localhost:8001")
	jwtSecret = []byte("test-secret-key")

	cleanup := func() {
		db = originalDB
		mockDB.Close()
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("AUTH_SERVICE_URL")
	}

	return mock, cleanup
}

// generateAdminToken creates a valid admin JWT
func generateAdminToken() string {
	token, _ := generateJWT(1, "admin", "admin")
	return token
}

// generateUserToken creates a valid user JWT
func generateUserToken() string {
	token, _ := generateJWT(2, "user", "user")
	return token
}

// createAdminRequest creates an HTTP request with admin authorization
func createAdminRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewBuffer(jsonBody))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+generateAdminToken())
	return req
}

// createRequestWithToken creates an HTTP request with a specific token
func createRequestWithToken(t *testing.T, method, path string, body interface{}, token string) *http.Request {
	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewBuffer(jsonBody))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// Get Users Handler Tests

// TestGetUsersHandler_Success verifies successful retrieval of user list
func TestGetUsersHandler_Success(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup mock data
	rows := sqlmock.NewRows([]string{"id", "username", "role", "created_at"}).
		AddRow(1, "admin", "admin", time.Now()).
		AddRow(2, "user1", "user", time.Now()).
		AddRow(3, "premium1", "premium", time.Now())

	mock.ExpectQuery("SELECT id, username, role, created_at FROM users ORDER BY id").
		WillReturnRows(rows)

	req := createAdminRequest(t, http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	getUsersHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	// Parse response
	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %v", len(users))
	}

	// Verify first user
	if users[0].Username != "admin" {
		t.Errorf("First user username = %v, want admin", users[0].Username)
	}

	// Verify content type
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("Content-Type should be application/json")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestGetUsersHandler_EmptyList verifies handling of empty user list
func TestGetUsersHandler_EmptyList(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup empty result
	rows := sqlmock.NewRows([]string{"id", "username", "role", "created_at"})
	mock.ExpectQuery("SELECT id, username, role, created_at FROM users ORDER BY id").
		WillReturnRows(rows)

	req := createAdminRequest(t, http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	getUsersHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(users) != 0 {
		t.Errorf("Expected 0 users, got %v", len(users))
	}
}

// TestGetUsersHandler_DatabaseError verifies handling of database errors
func TestGetUsersHandler_DatabaseError(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup mock to return error
	mock.ExpectQuery("SELECT id, username, role, created_at FROM users ORDER BY id").
		WillReturnError(sql.ErrConnDone)

	req := createAdminRequest(t, http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	getUsersHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusInternalServerError)
	}

	if !strings.Contains(w.Body.String(), "Failed to query users") {
		t.Errorf("Expected database error message, got: %v", w.Body.String())
	}
}

// Create User Handler Tests

// TestCreateUserHandler_Success verifies successful user creation
func TestCreateUserHandler_Success(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup mock expectations
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE username = \\$1\\)").
		WithArgs("newuser").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	mock.ExpectQuery("INSERT INTO users \\(username, password_hash, role\\) VALUES \\(\\$1, \\$2, \\$3\\) RETURNING id").
		WithArgs("newuser", sqlmock.AnyArg(), "user").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	reqBody := CreateUserRequest{
		Username: "newuser",
		Password: "password123",
		Role:     "user",
	}

	req := createAdminRequest(t, http.MethodPost, "/users", reqBody)
	w := httptest.NewRecorder()

	createUserHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusCreated)
	}

	var user User
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if user.Username != "newuser" {
		t.Errorf("Username = %v, want newuser", user.Username)
	}

	if user.Role != "user" {
		t.Errorf("Role = %v, want user", user.Role)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestCreateUserHandler_InvalidRole verifies rejection of invalid roles
func TestCreateUserHandler_InvalidRole(t *testing.T) {
	_, cleanup := setupAdminTestDB(t)
	defer cleanup()

	tests := []struct {
		name string
		role string
	}{
		{
			name: "Invalid role",
			role: "superuser",
		},
		{
			name: "Empty role",
			role: "",
		},
		{
			name: "Role with spaces",
			role: "admin user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := CreateUserRequest{
				Username: "testuser",
				Password: "password123",
				Role:     tt.role,
			}

			req := createAdminRequest(t, http.MethodPost, "/users", reqBody)
			w := httptest.NewRecorder()

			createUserHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Status code = %v, want %v", w.Code, http.StatusBadRequest)
			}

			if !strings.Contains(w.Body.String(), "Invalid role") {
				t.Errorf("Expected 'Invalid role' error, got: %v", w.Body.String())
			}
		})
	}
}

// TestCreateUserHandler_MissingFields verifies rejection of missing required fields
func TestCreateUserHandler_MissingFields(t *testing.T) {
	_, cleanup := setupAdminTestDB(t)
	defer cleanup()

	tests := []struct {
		name     string
		username string
		password string
	}{
		{
			name:     "Missing username",
			username: "",
			password: "password123",
		},
		{
			name:     "Missing password",
			username: "testuser",
			password: "",
		},
		{
			name:     "Both missing",
			username: "",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := CreateUserRequest{
				Username: tt.username,
				Password: tt.password,
				Role:     "user",
			}

			req := createAdminRequest(t, http.MethodPost, "/users", reqBody)
			w := httptest.NewRecorder()

			createUserHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Status code = %v, want %v", w.Code, http.StatusBadRequest)
			}

			if !strings.Contains(w.Body.String(), "required") {
				t.Errorf("Expected 'required' error message, got: %v", w.Body.String())
			}
		})
	}
}

// TestCreateUserHandler_DuplicateUser verifies rejection of duplicate username
func TestCreateUserHandler_DuplicateUser(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup mock to indicate user exists
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE username = \\$1\\)").
		WithArgs("existinguser").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	reqBody := CreateUserRequest{
		Username: "existinguser",
		Password: "password123",
		Role:     "user",
	}

	req := createAdminRequest(t, http.MethodPost, "/users", reqBody)
	w := httptest.NewRecorder()

	createUserHandler(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusConflict)
	}

	if !strings.Contains(w.Body.String(), "User already exists") {
		t.Errorf("Expected 'User already exists' error, got: %v", w.Body.String())
	}
}

// TestCreateUserHandler_AllValidRoles verifies user creation for all valid roles
func TestCreateUserHandler_AllValidRoles(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	roles := []string{"admin", "user", "premium"}

	for i, role := range roles {
		t.Run("Role_"+role, func(t *testing.T) {
			username := "user_" + role

			mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM users WHERE username = \\$1\\)").
				WithArgs(username).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

			mock.ExpectQuery("INSERT INTO users \\(username, password_hash, role\\) VALUES \\(\\$1, \\$2, \\$3\\) RETURNING id").
				WithArgs(username, sqlmock.AnyArg(), role).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(i + 1))

			reqBody := CreateUserRequest{
				Username: username,
				Password: "password123",
				Role:     role,
			}

			req := createAdminRequest(t, http.MethodPost, "/users", reqBody)
			w := httptest.NewRecorder()

			createUserHandler(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Status code = %v, want %v", w.Code, http.StatusCreated)
			}

			var user User
			if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
				t.Fatalf("Failed to unmarshal response body into User: %v", err)
			}

			if user.Role != role {
				t.Errorf("Role = %v, want %v", user.Role, role)
			}
		})
	}
}

// Edit User Handler Tests

// TestEditUserHandler_Success verifies successful user role update
func TestEditUserHandler_Success(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup mock expectations
	mock.ExpectQuery("SELECT username, role FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username", "role"}).AddRow("testuser", "user"))

	mock.ExpectExec("UPDATE users SET role = \\$1 WHERE id = \\$2").
		WithArgs("admin", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	reqBody := UpdateUserRequest{Role: "admin"}

	req := createAdminRequest(t, http.MethodPut, "/users/1", reqBody)
	w := httptest.NewRecorder()

	editUserHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	var user User
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if user.Role != "admin" {
		t.Errorf("Role = %v, want admin", user.Role)
	}

	if user.ID != 1 {
		t.Errorf("ID = %v, want 1", user.ID)
	}
}

// TestEditUserHandler_InvalidID verifies handling of invalid user ID
func TestEditUserHandler_InvalidID(t *testing.T) {
	_, cleanup := setupAdminTestDB(t)
	defer cleanup()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "Non-numeric ID",
			path: "/users/abc",
		},
		{
			name: "Mixed ID",
			path: "/users/123abc",
		},
		{
			name: "Empty ID",
			path: "/users/",
		},
		{
			name: "Negative ID",
			path: "/users/-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := UpdateUserRequest{Role: "admin"}
			req := createAdminRequest(t, http.MethodPut, tt.path, reqBody)
			w := httptest.NewRecorder()

			editUserHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Status code = %v, want %v", w.Code, http.StatusBadRequest)
			}

			if !strings.Contains(w.Body.String(), "Invalid user ID") {
				t.Errorf("Expected 'Invalid user ID' error, got: %v", w.Body.String())
			}
		})
	}
}

// TestEditUserHandler_UserNotFound verifies handling when user doesn't exist
func TestEditUserHandler_UserNotFound(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT username, role FROM users WHERE id = \\$1").
		WithArgs(999).
		WillReturnError(sql.ErrNoRows)

	reqBody := UpdateUserRequest{Role: "admin"}
	req := createAdminRequest(t, http.MethodPut, "/users/999", reqBody)
	w := httptest.NewRecorder()

	editUserHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusNotFound)
	}

	if !strings.Contains(w.Body.String(), "User not found") {
		t.Errorf("Expected 'User not found' error, got: %v", w.Body.String())
	}
}

// Delete User Handler Tests

// TestDeleteUserHandler_Success verifies successful user deletion
func TestDeleteUserHandler_Success(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	// Setup mock expectations
	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))

	mock.ExpectExec("DELETE FROM users WHERE id = \\$1").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := createAdminRequest(t, http.MethodDelete, "/users/1", nil)
	w := httptest.NewRecorder()

	deleteUserHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["message"] != "User deleted successfully" {
		t.Errorf("Message = %v, want 'User deleted successfully'", resp["message"])
	}

	if resp["deleted_user_id"] != float64(1) {
		t.Errorf("Deleted user ID = %v, want 1", resp["deleted_user_id"])
	}
}

// TestDeleteUserHandler_InvalidID verifies handling of invalid user ID
func TestDeleteUserHandler_InvalidID(t *testing.T) {
	_, cleanup := setupAdminTestDB(t)
	defer cleanup()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "Non-numeric ID",
			path: "/users/abc",
		},
		{
			name: "Mixed ID",
			path: "/users/123abc",
		},
		{
			name: "Empty ID",
			path: "/users/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createAdminRequest(t, http.MethodDelete, tt.path, nil)
			w := httptest.NewRecorder()

			deleteUserHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Status code = %v, want %v", w.Code, http.StatusBadRequest)
			}

			if !strings.Contains(w.Body.String(), "Invalid user ID") {
				t.Errorf("Expected 'Invalid user ID' error, got: %v", w.Body.String())
			}
		})
	}
}

// TestDeleteUserHandler_UserNotFound verifies handling when user doesn't exist
func TestDeleteUserHandler_UserNotFound(t *testing.T) {
	mock, cleanup := setupAdminTestDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT username FROM users WHERE id = \\$1").
		WithArgs(999).
		WillReturnError(sql.ErrNoRows)

	req := createAdminRequest(t, http.MethodDelete, "/users/999", nil)
	w := httptest.NewRecorder()

	deleteUserHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusNotFound)
	}

	if !strings.Contains(w.Body.String(), "User not found") {
		t.Errorf("Expected 'User not found' error, got: %v", w.Body.String())
	}
}
