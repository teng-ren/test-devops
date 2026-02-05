package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type CompletionRequest struct {
	Prompt    string `json:"prompt"`
	NPredict  int    `json:"n_predict"`
	Stream    bool   `json:"stream"`
	MaxTokens int    `json:"max_tokens"`
}

type CompletionResponse struct {
	Content          string `json:"content"`
	TokensEvaluated  int    `json:"tokens_evaluated"`
	TokensPredicted  int    `json:"tokens_predicted"`
	GenerationTimeMs int    `json:"generation_time_ms"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

func completionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	modelName := os.Getenv("MODEL_NAME")
	if modelName == "" {
		modelName = "mock-model"
	}

	// Simulate some processing time
	time.Sleep(500 * time.Millisecond)

	// Generate a mock response
	response := fmt.Sprintf("This is a mock response from %s. You asked: %q. "+
		"In a real deployment, this would be processed by an actual LLM model. "+
		"The mock service is working correctly!", modelName, truncate(req.Prompt, 50))

	resp := CompletionResponse{
		Content:          response,
		TokensEvaluated:  len(req.Prompt) / 4, // Rough estimate
		TokensPredicted:  len(response) / 4,
		GenerationTimeMs: 500,
	}

	log.Printf("Processed prompt: %q -> response length: %d", truncate(req.Prompt, 30), len(response))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func main() {
	modelName := os.Getenv("MODEL_NAME")
	if modelName == "" {
		modelName = "mock-model"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/completion", completionHandler)

	log.Printf("Mock LLM service (%s) running on :8000", modelName)
	log.Fatal(http.ListenAndServe(":8000", mux))
}
