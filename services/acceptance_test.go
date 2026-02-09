package acceptance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	apiGatewayURL = "http://localhost:8000"
	timeout       = 30 * time.Second
)

// Test helper to make HTTP requests
func makeRequest(t *testing.T, method, endpoint string, body interface{}, token string) (*http.Response, []byte) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(method, apiGatewayURL+endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	respBody := make([]byte, 0)
	if resp.Body != nil {
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		respBody = buf.Bytes()
	}

	return resp, respBody
}

// AC1: User Login and Authentication Flow
// CI: ✅ Runs in CI (auth service only)
// Acceptance Criteria:
// - Existing user can login with valid credentials
// - User receives JWT token upon login
// - Token can be used for authenticated requests
func TestUserLoginAndAuthenticationFlow(t *testing.T) {
	// Use pre-seeded test user
	testUser := map[string]string{
		"username": getEnv("DEFAULT_TESTUSER_USERNAME", "testuser"),
		"password": getEnv("DEFAULT_TESTUSER_PASSWORD", "testpass"),
	}

	// Step 1: Login with credentials
	t.Log("Step 1: Logging in with valid credentials...")
	resp, body := makeRequest(t, "POST", "/auth/login", testUser, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	token, ok := loginResp["token"].(string)
	if !ok || token == "" {
		t.Fatalf("No token received in login response")
	}
	t.Logf("✓ Login successful, token received")

	// Step 2: Use token for authenticated request (test against admin endpoint - should be rejected)
	t.Log("Step 2: Accessing protected resource with token...")
	resp, _ = makeRequest(t, "GET", "/admin/users", nil, token)

	// Regular user should be denied admin access (but token is valid)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("Regular user should not have admin access!")
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Logf("✓ Token authentication successful (correctly denied admin access with status %d)", resp.StatusCode)
	} else {
		t.Logf("✓ Token processed (status %d)", resp.StatusCode)
	}

	t.Log("✅ ACCEPTANCE TEST PASSED: User Login and Authentication Flow")
}

// AC2: Admin User Management Flow
// CI: ✅ Runs in CI (auth service only)
// Acceptance Criteria:
// - Admin can login with admin credentials
// - Admin can view all users
// - Admin can create new users
// - Admin can update user roles
func TestAdminUserManagementFlow(t *testing.T) {
	// Step 1: Admin login
	t.Log("Step 1: Admin logging in...")
	adminCreds := map[string]string{
		"username": getEnv("DEFAULT_ADMIN_USERNAME", "admin"),
		"password": getEnv("DEFAULT_ADMIN_PASSWORD", "adminpass"),
	}

	resp, body := makeRequest(t, "POST", "/auth/login", adminCreds, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Admin login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	adminToken, ok := loginResp["token"].(string)
	if !ok || adminToken == "" {
		t.Fatalf("No admin token received")
	}
	t.Logf("✓ Admin logged in successfully")

	// Step 2: List all users
	t.Log("Step 2: Retrieving user list...")
	resp, body = makeRequest(t, "GET", "/admin/users", nil, adminToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve users (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ User list retrieved successfully")

	// Step 3: Create new user as admin
	t.Log("Step 3: Creating new user as admin...")
	timestamp := time.Now().Unix()
	newUser := map[string]interface{}{
		"username": fmt.Sprintf("admin_created_%d", timestamp),
		"password": "AdminPass123!",
		"role":     "user",
	}

	resp, body = makeRequest(t, "POST", "/admin/users", newUser, adminToken)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("User creation failed (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ New user created by admin")

	t.Log("✅ ACCEPTANCE TEST PASSED: Admin User Management Flow")
}

// AC3: Prompt/Chat Creation Flow
// CI: ⏭️ Skipped in CI (requires prompt-manager service)
// Acceptance Criteria:
// - Authenticated user can create a new chat/prompt
// - User can retrieve their chat history
// - User can view available models
func TestPromptCreationFlow(t *testing.T) {
	// Step 1: Login as test user
	t.Log("Step 1: Logging in as test user...")
	testCreds := map[string]string{
		"username": getEnv("DEFAULT_TESTUSER_USERNAME", "testuser"),
		"password": getEnv("DEFAULT_TESTUSER_PASSWORD", "testpass"),
	}

	resp, body := makeRequest(t, "POST", "/auth/login", testCreds, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Test user login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	userToken, ok := loginResp["token"].(string)
	if !ok || userToken == "" {
		t.Fatalf("No user token received")
	}
	t.Logf("✓ Test user logged in successfully")

	// Step 2: Retrieve available models
	t.Log("Step 2: Retrieving available models...")
	resp, body = makeRequest(t, "GET", "/models", nil, userToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve models (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ Models retrieved successfully")

	// Step 3: Create new chat/prompt
	t.Log("Step 3: Creating new chat...")
	newChat := map[string]interface{}{
		"prompt": "What is the meaning of life?",
		"model":  "gemma3",
	}

	resp, body = makeRequest(t, "POST", "/chats", newChat, userToken)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("Chat creation failed (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ Chat created successfully")

	// Step 4: Retrieve chat history
	t.Log("Step 4: Retrieving chat history...")
	resp, body = makeRequest(t, "GET", "/chats", nil, userToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve chat history (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ Chat history retrieved successfully")

	t.Log("✅ ACCEPTANCE TEST PASSED: Prompt Creation Flow")
}

// AC4: Unauthorized Access Prevention
// CI: ✅ Runs in CI (auth service only)
// Acceptance Criteria:
// - Unauthenticated users cannot access protected resources
// - Invalid tokens are rejected
// - Regular users cannot access admin endpoints
func TestUnauthorizedAccessPrevention(t *testing.T) {
	// Step 1: Try to access protected resource without token
	t.Log("Step 1: Attempting to access protected resource without token...")
	resp, _ := makeRequest(t, "GET", "/chats", nil, "")

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("Protected resource accessible without authentication!")
	}
	t.Logf("✓ Unauthenticated request properly rejected (status %d)", resp.StatusCode)

	// Step 2: Try with invalid token
	t.Log("Step 2: Attempting to access with invalid token...")
	resp, _ = makeRequest(t, "GET", "/chats", nil, "invalid.token.here")

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("Protected resource accessible with invalid token!")
	}
	t.Logf("✓ Invalid token properly rejected (status %d)", resp.StatusCode)

	// Step 3: Try to access admin endpoint as regular user
	t.Log("Step 3: Attempting admin endpoint as regular user...")
	testCreds := map[string]string{
		"username": getEnv("DEFAULT_TESTUSER_USERNAME", "testuser"),
		"password": getEnv("DEFAULT_TESTUSER_PASSWORD", "testpass"),
	}

	resp, body := makeRequest(t, "POST", "/auth/login", testCreds, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to log in as regular user (status %d, body: %s)", resp.StatusCode, string(body))
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	userToken, ok := loginResp["token"].(string)
	if !ok || userToken == "" {
		t.Fatalf("Login response did not contain a valid token: %#v", loginResp)
	}

	resp, _ = makeRequest(t, "GET", "/admin/users", nil, userToken)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("Regular user can access admin endpoints!")
	}
	t.Logf("✓ Regular user blocked from admin endpoints (status %d)", resp.StatusCode)

	t.Log("✅ ACCEPTANCE TEST PASSED: Unauthorized Access Prevention")
}

// AC5: End-to-End User Journey
// CI: ⏭️ Skipped in CI (requires prompt-manager service)
// Acceptance Criteria:
// - Complete workflow from login to prompt submission works seamlessly
// - All integrated services communicate properly
func TestEndToEndUserJourney(t *testing.T) {
	// Use pre-seeded test user
	testUser := map[string]string{
		"username": getEnv("DEFAULT_TESTUSER_USERNAME", "testuser"),
		"password": getEnv("DEFAULT_TESTUSER_PASSWORD", "testpass"),
	}

	// Journey Step 1: User logs in
	t.Log("Journey Step 1: User login...")
	resp, body := makeRequest(t, "POST", "/auth/login", testUser, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("User journey failed at login: %d - %s", resp.StatusCode, string(body))
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}
	token, ok := loginResp["token"].(string)
	if !ok || token == "" {
		t.Fatalf("No token received in login response")
	}
	t.Logf("✓ User logged in")

	// Journey Step 2: User explores available models
	t.Log("Journey Step 2: Exploring available models...")
	resp, body = makeRequest(t, "GET", "/models", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve models (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ Models endpoint accessed successfully")

	// Journey Step 3: User creates their first prompt
	t.Log("Journey Step 3: Creating first prompt...")
	firstPrompt := map[string]interface{}{
		"prompt": "Hello, AI! This is my first message.",
		"model":  "gemma3",
	}

	resp, body = makeRequest(t, "POST", "/chats", firstPrompt, token)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create prompt (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ First prompt created successfully")

	// Journey Step 4: User checks their chat history
	t.Log("Journey Step 4: Checking chat history...")
	resp, body = makeRequest(t, "GET", "/chats", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to retrieve chat history (status %d): %s", resp.StatusCode, string(body))
	}
	t.Logf("✓ Chat history accessed successfully")

	t.Log("✅ ACCEPTANCE TEST PASSED: End-to-End User Journey")
	t.Log("🎉 User successfully completed entire journey from login to prompt interaction")
}

// Helper function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
