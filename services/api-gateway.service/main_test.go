package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSHeaders(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		expectedOrigin  string
		expectedMethods string
		expectedHeaders string
	}{
		{
			name:            "GET request CORS headers",
			method:          http.MethodGet,
			expectedOrigin:  "*",
			expectedMethods: "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders: "Content-Type, Authorization",
		},
		{
			name:            "POST request CORS headers",
			method:          http.MethodPost,
			expectedOrigin:  "*",
			expectedMethods: "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders: "Content-Type, Authorization",
		},
		{
			name:            "PUT request CORS headers",
			method:          http.MethodPut,
			expectedOrigin:  "*",
			expectedMethods: "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders: "Content-Type, Authorization",
		},
		{
			name:            "DELETE request CORS headers",
			method:          http.MethodDelete,
			expectedOrigin:  "*",
			expectedMethods: "GET, POST, PUT, DELETE, OPTIONS",
			expectedHeaders: "Content-Type, Authorization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			corsHandler := corsMiddleware(handler)

			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			corsHandler.ServeHTTP(w, req)

			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tt.expectedOrigin {
				t.Errorf("Access-Control-Allow-Origin = %v, want %v", got, tt.expectedOrigin)
			}
			if got := w.Header().Get("Access-Control-Allow-Methods"); got != tt.expectedMethods {
				t.Errorf("Access-Control-Allow-Methods = %v, want %v", got, tt.expectedMethods)
			}
			if got := w.Header().Get("Access-Control-Allow-Headers"); got != tt.expectedHeaders {
				t.Errorf("Access-Control-Allow-Headers = %v, want %v", got, tt.expectedHeaders)
			}
		})
	}
}

// TestReverseProxy_Success verifies successful proxy forwarding
func TestReverseProxy_Success(t *testing.T) {
	// Create a mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "backend response"}`))
	}))
	defer backend.Close()

	// Create reverse proxy
	proxy := reverseProxy(backend.URL)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	w := httptest.NewRecorder()

	// Execute proxy
	proxy.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "backend response") {
		t.Errorf("Expected backend response, got: %v", body)
	}
}

// TestReverseProxy_BackendError verifies handling when backend is unavailable
func TestReverseProxy_BackendError(t *testing.T) {
	// Create a backend that will be closed immediately
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	backend.Close()

	proxy := reverseProxy(backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// This should handle the error gracefully
	proxy.ServeHTTP(w, req)

	// The proxy should return an error (5xx) response when backend is unavailable
	if w.Code < 500 || w.Code >= 600 {
		t.Errorf("expected 5xx error status when backend is unavailable, got %d", w.Code)
	}
}
