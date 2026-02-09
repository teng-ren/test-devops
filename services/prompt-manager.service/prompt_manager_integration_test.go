package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/pubsub"
	_ "github.com/lib/pq"
)

// globalTestMutex protects access to global variables across all tests
var globalTestMutex sync.Mutex

// testContext holds test-specific state to avoid modifying global state
type testContext struct {
	db             *sql.DB
	authURL        string
	pubsubClient   *pubsub.Client
	llmServiceURLs map[string]string
}

// setupTestContext creates an isolated test environment
func setupTestContext(t *testing.T) *testContext {
	// Database connection
	connStr := "host=" + getEnv("DB_HOST", "localhost") +
		" port=" + getEnv("DB_PORT", "5433") +
		" user=" + getEnv("DB_USER", "postgres") +
		" password=" + getEnv("DB_PASSWORD", "postgres") +
		" dbname=" + getEnv("DB_NAME", "chats_db") +
		" sslmode=disable"

	testDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := testDB.Ping(); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}

	// Setup Pub/Sub client (optional - only fail if test explicitly needs it)
	ctx := context.Background()
	projectID := getEnv("PUBSUB_PROJECT_ID", "local-project")

	psClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		// Don't fail here - let tests that need Pub/Sub handle the error
		t.Logf("Warning: Failed to create Pub/Sub client: %v (tests requiring Pub/Sub will be skipped)", err)
		psClient = nil
	}

	return &testContext{
		db:           testDB,
		authURL:      getEnv("AUTH_SERVICE_URL", "http://localhost:8001"),
		pubsubClient: psClient,
		llmServiceURLs: map[string]string{
			"gemma3": getEnv("GEMMA3_SERVICE_URL", "http://localhost:8003"),
			"qwen3":  getEnv("QWEN3_SERVICE_URL", "http://localhost:8004"),
		},
	}
}

// withTestContext temporarily sets global variables for a single handler execution
func (tc *testContext) withTestContext(fn func()) {
	globalTestMutex.Lock()
	defer globalTestMutex.Unlock()

	// Save original global state
	originalDB := db
	originalAuthURL := authServiceURL
	originalPubsubClient := pubsubClient
	originalTopic := topic
	originalLLMServiceURLs := llmServiceURLs

	// Set test-specific state
	db = tc.db
	authServiceURL = tc.authURL
	pubsubClient = tc.pubsubClient
	llmServiceURLs = tc.llmServiceURLs

	// Execute the handler
	fn()

	// Restore original state
	db = originalDB
	authServiceURL = originalAuthURL
	pubsubClient = originalPubsubClient
	topic = originalTopic
	llmServiceURLs = originalLLMServiceURLs
}

// Close cleans up the test context
func (tc *testContext) Close() {
	if tc.db != nil {
		tc.db.Close()
	}
	if tc.pubsubClient != nil {
		tc.pubsubClient.Close()
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// generateTestJWT creates a simple JWT token for testing
// Format: header.payload.signature (we don't verify signature in tests)
func generateTestJWT(userID int, username, role string) string {
	// JWT Header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// JWT Payload (Claims)
	payload := map[string]interface{}{
		"user_id":  userID,
		"username": username,
		"role":     role,
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Simple signature (not validated in tests)
	signature := base64.RawURLEncoding.EncodeToString([]byte("test-signature"))

	return fmt.Sprintf("%s.%s.%s", headerB64, payloadB64, signature)
}

// cleanupTestChat removes a test chat and its messages
func cleanupTestChat(t *testing.T, tc *testContext, chatID string) {
	_, err := tc.db.Exec("DELETE FROM messages WHERE chat_id = $1", chatID)
	if err != nil {
		t.Logf("Warning: Failed to cleanup messages for chat %s: %v", chatID, err)
	}
	_, err = tc.db.Exec("DELETE FROM chats WHERE id = $1", chatID)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test chat %s: %v", chatID, err)
	}
}

// callProtectedHandler wraps handler with auth middleware for testing protected endpoints
func callProtectedHandler(tc *testContext, handler http.HandlerFunc, rr *httptest.ResponseRecorder, req *http.Request) {
	tc.withTestContext(func() {
		authMiddleware(handler)(rr, req)
	})
}

// mockAuthServer creates a test auth server
func mockAuthServer() *httptest.Server {
	mux := http.NewServeMux()

	// Mock /validate endpoint
	mux.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req AuthValidateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate JWT tokens (check for 3 parts separated by dots: header.payload.signature)
		parts := strings.Split(req.Token, ".")
		if len(parts) == 3 && req.Token != "invalid-token" {
			// For any valid-looking JWT, return success
			json.NewEncoder(w).Encode(AuthValidateResponse{
				Valid:    true,
				UserID:   123,
				Username: "testuser",
				Role:     "user",
			})
		} else {
			json.NewEncoder(w).Encode(AuthValidateResponse{
				Valid: false,
			})
		}
	})

	return httptest.NewServer(mux)
}

// mockLLMServer creates a test LLM server
func mockLLMServer(modelName string) *httptest.Server {
	mux := http.NewServeMux()

	// Mock /completion endpoint
	mux.HandleFunc("/completion", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req LlamaCppRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Generate a response based on the model
		content := fmt.Sprintf("This is a test response from %s model.", modelName)

		json.NewEncoder(w).Encode(LlamaCppResponse{
			Content:         content,
			TokensEvaluated: 10,
			TokensPredicted: 20,
		})
	})

	// Mock /health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	return httptest.NewServer(mux)
}

// Test: Health Endpoint
func TestHealthHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	tc.withTestContext(func() {
		healthHandler(rr, req)
	})

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", resp.Status)
	}

	if resp.Service != "prompt-manager" {
		t.Errorf("Expected service 'prompt-manager', got '%s'", resp.Service)
	}
}

// Test: Models Endpoint
func TestModelsHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	rr := httptest.NewRecorder()

	tc.withTestContext(func() {
		modelsHandler(rr, req)
	})

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check if models data is present
	modelsData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	models, ok := modelsData["models"].([]interface{})
	if !ok {
		t.Fatal("Expected models to be an array")
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}
}

// Test: Create Chat - Success
func TestCreateChatHandler_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	createReq := CreateChatRequest{
		Model: "gemma3",
	}
	body, _ := json.Marshal(createReq)

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodPost, "/chats", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusCreated, rr.Body.String())
	}

	var resp APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Extract chat from response
	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	chatData, ok := respData["chat"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected chat data in response")
	}

	chatID, ok := chatData["id"].(string)
	if !ok || chatID == "" {
		t.Error("Expected chat ID in response")
	}

	// Cleanup
	defer cleanupTestChat(t, tc, chatID)

	// Verify in database
	var dbModel string
	err := tc.db.QueryRow("SELECT model FROM chats WHERE id = $1", chatID).Scan(&dbModel)
	if err != nil {
		t.Fatalf("Failed to query created chat: %v", err)
	}

	if dbModel != "gemma3" {
		t.Errorf("Expected model 'gemma3', got '%s'", dbModel)
	}
}

// Test: Create Chat - Unauthorized
func TestCreateChatHandler_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	createReq := CreateChatRequest{
		Model: "gemma3",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/chats", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

// Test: Create Chat - Invalid Model
func TestCreateChatHandler_InvalidModel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	createReq := CreateChatRequest{
		Model: "invalid-model",
	}
	body, _ := json.Marshal(createReq)

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodPost, "/chats", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

// Test: List Chats - Success
func TestListChatsHandler_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	// Create a test chat
	testUserID := 123
	var chatID string
	err := tc.db.QueryRow(
		"INSERT INTO chats (user_id, model, title) VALUES ($1, $2, $3) RETURNING id",
		testUserID, "gemma3", "Test Chat",
	).Scan(&chatID)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}
	defer cleanupTestChat(t, tc, chatID)

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp ChatListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Chats == nil {
		t.Error("Expected chats array in response")
	}

	if resp.Page != 1 {
		t.Errorf("Expected page 1, got %d", resp.Page)
	}
}

// Test: Pagination - List Chats with Page Size
func TestListChatsHandler_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	// Create multiple test chats
	testUserID := 123
	chatIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		var chatID string
		err := tc.db.QueryRow(
			"INSERT INTO chats (user_id, model, title) VALUES ($1, $2, $3) RETURNING id",
			testUserID, "gemma3", fmt.Sprintf("Test Chat %d", i+1),
		).Scan(&chatID)
		if err != nil {
			t.Fatalf("Failed to create test chat: %v", err)
		}
		chatIDs[i] = chatID
		defer cleanupTestChat(t, tc, chatID)
	}

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodGet, "/chats?page=1&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp ChatListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.PageSize != 2 {
		t.Errorf("Expected page size 2, got %d", resp.PageSize)
	}

	if resp.TotalCount < 3 {
		t.Errorf("Expected total count >= 3, got %d", resp.TotalCount)
	}
}

// Test: Get Chat - Success
func TestGetChatHandler_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	// Create a test chat
	testUserID := 123
	var chatID string
	err := tc.db.QueryRow(
		"INSERT INTO chats (user_id, model, title) VALUES ($1, $2, $3) RETURNING id",
		testUserID, "gemma3", "Test Chat",
	).Scan(&chatID)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}
	defer cleanupTestChat(t, tc, chatID)

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodGet, "/chats/"+chatID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp ChatWithMessages
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Chat.ID != chatID {
		t.Errorf("Expected chat ID '%s', got '%s'", chatID, resp.Chat.ID)
	}

	if resp.Messages == nil {
		t.Error("Expected messages array in response")
	}
}

// Test: Get Chat - Not Found
func TestGetChatHandler_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodGet, "/chats/99999999-0000-0000-0000-000000000000", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}
}

// Test: Delete Chat - Success
func TestDeleteChatHandler_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	// Create a test chat
	testUserID := 123
	var chatID string
	err := tc.db.QueryRow(
		"INSERT INTO chats (user_id, model, title) VALUES ($1, $2, $3) RETURNING id",
		testUserID, "gemma3", "Test Chat",
	).Scan(&chatID)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodDelete, "/chats/"+chatID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	callProtectedHandler(tc, chatsHandler, rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusNoContent)
	}

	// Verify deletion
	var count int
	err = tc.db.QueryRow("SELECT COUNT(*) FROM chats WHERE id = $1", chatID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to verify deletion: %v", err)
	}

	if count != 0 {
		t.Error("Chat was not deleted from database")
	}
}

// Test: Send Message - Success
func TestSendMessageHandler_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	if tc.pubsubClient == nil {
		t.Skip("Pub/Sub client not available - skipping test")
	}

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	// Create a test chat
	testUserID := 123
	var chatID string
	err := tc.db.QueryRow(
		"INSERT INTO chats (user_id, model, title) VALUES ($1, $2, $3) RETURNING id",
		testUserID, "gemma3", "Test Chat",
	).Scan(&chatID)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}
	defer cleanupTestChat(t, tc, chatID)

	// Setup topic for Pub/Sub (if needed)
	ctx := context.Background()
	topicID := getEnv("PUBSUB_TOPIC_ID", "prompt-requests")
	testTopic := tc.pubsubClient.Topic(topicID)
	exists, err := testTopic.Exists(ctx)
	if err != nil {
		t.Fatalf("Failed to check if Pub/Sub topic %q exists: %v", topicID, err)
	}
	if !exists {
		testTopic, err = tc.pubsubClient.CreateTopic(ctx, topicID)
		if err != nil {
			t.Fatalf("Failed to create Pub/Sub topic %q: %v", topicID, err)
		}
	}

	sendReq := SendMessageRequest{
		Content: "Hello, test message",
	}
	body, _ := json.Marshal(sendReq)

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodPost, "/chats/"+chatID+"/messages", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	tc.withTestContext(func() {
		// Save and restore the global topic to avoid leaking state across tests
		oldTopic := topic
		defer func() {
			topic = oldTopic
		}()

		// Initialize topic for the test
		topic = testTopic
		authMiddleware(chatsHandler)(rr, req)
	})

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusCreated, rr.Body.String())
	}

	var resp APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify message was created in database
	var msgCount int
	err = tc.db.QueryRow("SELECT COUNT(*) FROM messages WHERE chat_id = $1", chatID).Scan(&msgCount)
	if err != nil {
		t.Fatalf("Failed to query messages: %v", err)
	}

	if msgCount == 0 {
		t.Error("Expected message to be created in database")
	}
}

// Test: LLM Integration - Process Message
func TestProcessMessage_LLMIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock LLM server
	mockLLM := mockLLMServer("gemma3")
	defer mockLLM.Close()
	tc.llmServiceURLs["gemma3"] = mockLLM.URL

	// Create a test chat
	testUserID := 123
	var chatID string
	err := tc.db.QueryRow(
		"INSERT INTO chats (user_id, model, title) VALUES ($1, $2, $3) RETURNING id",
		testUserID, "gemma3", "Test Chat",
	).Scan(&chatID)
	if err != nil {
		t.Fatalf("Failed to create test chat: %v", err)
	}
	defer cleanupTestChat(t, tc, chatID)

	// Create a user message
	var msgID string
	err = tc.db.QueryRow(
		"INSERT INTO messages (chat_id, role, content, status) VALUES ($1, $2, $3, $4) RETURNING id",
		chatID, RoleUser, "Test question", StatusCompleted,
	).Scan(&msgID)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}

	// Process the message
	pubsubMsg := PubSubMessage{
		ChatID:    chatID,
		MessageID: msgID,
		UserID:    testUserID,
		Model:     "gemma3",
	}

	tc.withTestContext(func() {
		err = processMessage(pubsubMsg)
	})

	if err != nil {
		t.Errorf("processMessage failed: %v", err)
	}

	// Verify assistant message was created
	var assistantMsgCount int
	err = tc.db.QueryRow(
		"SELECT COUNT(*) FROM messages WHERE chat_id = $1 AND role = $2",
		chatID, RoleAssistant,
	).Scan(&assistantMsgCount)
	if err != nil {
		t.Fatalf("Failed to query assistant messages: %v", err)
	}

	if assistantMsgCount == 0 {
		t.Error("Expected assistant message to be created")
	}

	// Verify assistant message has content
	var content string
	var status MessageStatus
	err = tc.db.QueryRow(
		"SELECT content, status FROM messages WHERE chat_id = $1 AND role = $2 ORDER BY created_at DESC LIMIT 1",
		chatID, RoleAssistant,
	).Scan(&content, &status)
	if err != nil {
		t.Fatalf("Failed to query assistant message: %v", err)
	}

	if content == "" {
		t.Error("Expected assistant message to have content")
	}

	if status != StatusCompleted {
		t.Errorf("Expected status 'completed', got '%s'", status)
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

// Test: Pub/Sub Connection
func TestPubSubConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	if tc.pubsubClient == nil {
		t.Skip("Pub/Sub client not available - skipping test")
	}

	ctx := context.Background()
	topicID := getEnv("PUBSUB_TOPIC_ID", "prompt-requests")

	testTopic := tc.pubsubClient.Topic(topicID)
	exists, err := testTopic.Exists(ctx)

	if err != nil {
		t.Fatalf("Failed to check topic existence: %v", err)
	}

	if !exists {
		// Try to create the topic
		_, err = tc.pubsubClient.CreateTopic(ctx, topicID)
		if err != nil {
			t.Fatalf("Failed to create test topic: %v", err)
		}
	}

	// Topic exists or was created successfully
	t.Logf("Pub/Sub topic '%s' is accessible", topicID)
}

// Test: Auth Middleware - Valid Token
func TestAuthMiddleware_ValidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	// Create a protected handler
	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{"user_id": userID})
	})

	token := generateTestJWT(123, "testuser", "user")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()

	tc.withTestContext(func() {
		handler(rr, req)
	})

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

// Test: Auth Middleware - Invalid Token
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tc := setupTestContext(t)
	defer tc.Close()

	// Setup mock auth server
	mockAuth := mockAuthServer()
	defer mockAuth.Close()
	tc.authURL = mockAuth.URL

	// Create a protected handler
	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rr := httptest.NewRecorder()

	tc.withTestContext(func() {
		handler(rr, req)
	})

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}
