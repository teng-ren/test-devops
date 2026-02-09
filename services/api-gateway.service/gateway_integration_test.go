package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Mock auth service for testing
func mockAuthService() *httptest.Server {
	mux := http.NewServeMux()

	// Mock login endpoint
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": "mock-jwt-token-12345",
		})
	})

	// Mock validate endpoint
	mux.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": true,
			"role":  "admin",
		})
	})

	// Mock users endpoint
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		users := []map[string]interface{}{
			{"id": 1, "username": "admin", "role": "admin"},
			{"id": 2, "username": "user1", "role": "user"},
		}
		json.NewEncoder(w).Encode(users)
	})

	return httptest.NewServer(mux)
}

// Test: CORS Headers
func TestCORSMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with CORS middleware
	corsHandler := corsMiddleware(handler)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	// Execute
	corsHandler.ServeHTTP(rr, req)

	// Assert CORS headers
	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got: %s", origin)
	}

	expectedMethods := "GET, POST, PUT, DELETE, OPTIONS"
	if methods := rr.Header().Get("Access-Control-Allow-Methods"); methods != expectedMethods {
		t.Errorf("Expected Access-Control-Allow-Methods: %s, got: %s", expectedMethods, methods)
	}

	expectedHeaders := "Content-Type, Authorization"
	if headers := rr.Header().Get("Access-Control-Allow-Headers"); headers != expectedHeaders {
		t.Errorf("Expected Access-Control-Allow-Headers: %s, got: %s", expectedHeaders, headers)
	}
}

// Test: CORS Preflight Request
func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with CORS middleware
	corsHandler := corsMiddleware(handler)

	// Create OPTIONS request (preflight)
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rr := httptest.NewRecorder()

	// Execute
	corsHandler.ServeHTTP(rr, req)

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code for preflight: got %v want %v", status, http.StatusOK)
	}

	// Verify CORS headers are set
	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got: %s", origin)
	}
}

// Test: Reverse Proxy to Auth Service
func TestReverseProxy_AuthService(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Create mock auth service
	mockServer := mockAuthService()
	defer mockServer.Close()

	// Set up environment
	os.Setenv("AUTH_SERVICE_URL", mockServer.URL)
	defer os.Unsetenv("AUTH_SERVICE_URL")

	// Create gateway mux
	mux := http.NewServeMux()
	mux.Handle("/auth/", http.StripPrefix("/auth", reverseProxy(mockServer.URL)))

	// Create login request
	loginReq := map[string]string{
		"username": "testuser",
		"password": "testpass",
	}
	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	// Execute
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Proxy returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp map[string]string
	err := json.NewDecoder(rr.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if token, exists := resp["token"]; !exists || token == "" {
		t.Error("Expected token in response")
	}
}

// Test: Gateway Routes to Admin Endpoints
func TestGateway_AdminRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Create mock auth service
	mockServer := mockAuthService()
	defer mockServer.Close()

	// Set up environment
	os.Setenv("AUTH_SERVICE_URL", mockServer.URL)
	defer os.Unsetenv("AUTH_SERVICE_URL")

	// Create gateway mux
	mux := http.NewServeMux()
	mux.Handle("/admin/", http.StripPrefix("/admin", reverseProxy(mockServer.URL)))

	// Create request to get users
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer mock-token")

	// Execute
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Assert
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Proxy returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var users []map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&users)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(users) < 1 {
		t.Error("Expected at least one user in response")
	}
}

// Test: Full Gateway Flow with CORS
func TestGateway_FullFlowWithCORS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Create mock auth service
	mockServer := mockAuthService()
	defer mockServer.Close()

	// Set up environment
	os.Setenv("AUTH_SERVICE_URL", mockServer.URL)
	defer os.Unsetenv("AUTH_SERVICE_URL")

	// Create gateway mux with CORS
	mux := http.NewServeMux()
	mux.Handle("/auth/", http.StripPrefix("/auth", reverseProxy(mockServer.URL)))
	handler := corsMiddleware(mux)

	// Create login request
	loginReq := map[string]string{
		"username": "testuser",
		"password": "testpass",
	}
	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5173")

	// Execute
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Assert status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Assert CORS headers
	if origin := rr.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected CORS header, got: %s", origin)
	}

	// Assert response body
	var resp map[string]string
	err := json.NewDecoder(rr.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if token, exists := resp["token"]; !exists || token == "" {
		t.Error("Expected token in response")
	}
}

// Test: Invalid Route
func TestGateway_InvalidRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Create mock auth service
	mockServer := mockAuthService()
	defer mockServer.Close()

	// Set up environment
	os.Setenv("AUTH_SERVICE_URL", mockServer.URL)
	defer os.Unsetenv("AUTH_SERVICE_URL")

	// Create gateway mux
	mux := http.NewServeMux()
	mux.Handle("/auth/", http.StripPrefix("/auth", reverseProxy(mockServer.URL)))

	// Create request to invalid route
	req := httptest.NewRequest(http.MethodGet, "/invalid/route", nil)

	// Execute
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Assert - should return 404
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected status 404 for invalid route, got: %v", status)
	}
}

// Test: Environment Variable Configuration
func TestGateway_EnvironmentConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// Test that AUTH_SERVICE_URL can be set
	expectedURL := "http://test-auth-service:8001"
	os.Setenv("AUTH_SERVICE_URL", expectedURL)
	defer os.Unsetenv("AUTH_SERVICE_URL")

	actualURL := os.Getenv("AUTH_SERVICE_URL")
	if actualURL != expectedURL {
		t.Errorf("Expected AUTH_SERVICE_URL: %s, got: %s", expectedURL, actualURL)
	}
}
