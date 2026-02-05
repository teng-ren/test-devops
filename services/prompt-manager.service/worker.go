package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
)

var (
	llmServiceURLs map[string]string
	workerCtx      context.Context
	workerCancel   context.CancelFunc
	workerWg       sync.WaitGroup
)

// LlamaCppRequest is the request format for llama.cpp server
type LlamaCppRequest struct {
	Prompt    string `json:"prompt"`
	NPredict  int    `json:"n_predict,omitempty"`
	Stream    bool   `json:"stream"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// LlamaCppResponse is the response format from llama.cpp server
type LlamaCppResponse struct {
	Content          string `json:"content"`
	TokensEvaluated  int    `json:"tokens_evaluated"`
	TokensPredicted  int    `json:"tokens_predicted"`
	GenerationTimeMs int    `json:"generation_time_ms"`
}

func initWorker() {
	// Initialize LLM service URLs from environment
	llmServiceURLs = make(map[string]string)

	gemmaURL := os.Getenv("GEMMA3_SERVICE_URL")
	if gemmaURL == "" {
		gemmaURL = "http://gemma3:8000"
	}
	llmServiceURLs["gemma3"] = gemmaURL

	qwenURL := os.Getenv("QWEN3_SERVICE_URL")
	if qwenURL == "" {
		qwenURL = "http://qwen3:8000"
	}
	llmServiceURLs["qwen3"] = qwenURL

	log.Printf("LLM Service URLs: %v", llmServiceURLs)
}

// startWorker starts the Pub/Sub message consumer
func startWorker() {
	workerCtx, workerCancel = context.WithCancel(context.Background())
	workerWg.Add(1)

	go func() {
		defer workerWg.Done()

		subscriptionID := os.Getenv("PUBSUB_SUBSCRIPTION_ID")
		sub := pubsubClient.Subscription(subscriptionID)
		sub.ReceiveSettings.MaxOutstandingMessages = 10
		sub.ReceiveSettings.NumGoroutines = 5

		log.Println("Worker started, listening for messages...")

		err := sub.Receive(workerCtx, func(ctx context.Context, msg *pubsub.Message) {
			log.Printf("Received message: %s", string(msg.Data))

			var pubSubMsg PubSubMessage
			if err := json.Unmarshal(msg.Data, &pubSubMsg); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				msg.Nack()
				return
			}

			// Process the message
			if err := processMessage(pubSubMsg); err != nil {
				log.Printf("Error processing message %s: %v", pubSubMsg.MessageID, err)
			}

			msg.Ack()
		})

		if err != nil && err != context.Canceled {
			log.Printf("Worker error: %v", err)
		}

		log.Println("Worker stopped")
	}()
}

// stopWorker stops the Pub/Sub message consumer
func stopWorker() {
	if workerCancel != nil {
		workerCancel()
	}
	workerWg.Wait()
}

// processMessage processes a single chat message event
func processMessage(msg PubSubMessage) error {
	// 1. Create a placeholder "pending" assistant message
	assistantMsg, err := CreateMessage(msg.ChatID, RoleAssistant, "")
	if err != nil {
		return fmt.Errorf("failed to create assistant message: %w", err)
	}

	// 2. Retrieve chat history
	messages, err := GetMessages(msg.ChatID)
	if err != nil {
		failMsg := "Failed to retrieve chat history"
		_ = UpdateMessageStatus(assistantMsg.ID, StatusFailed, nil, nil, &failMsg)
		return fmt.Errorf("failed to get messages: %w", err)
	}

	// 3. Construct prompt from history
	prompt := buildPromptFromHistory(messages)

	// 4. Get LLM service URL
	llmURL, ok := llmServiceURLs[msg.Model]
	if !ok {
		failMsg := fmt.Sprintf("Unknown model: %s", msg.Model)
		_ = UpdateMessageStatus(assistantMsg.ID, StatusFailed, nil, nil, &failMsg)
		return fmt.Errorf("%s", failMsg)
	}

	// 5. Call LLM service
	// Note: We use the assistantMsg ID for logging/tracking if needed
	log.Printf("Calling LLM for chat %s, assistant message %s", msg.ChatID, assistantMsg.ID)
	responseContent, tokensUsed, err := callLLMService(llmURL, prompt)
	if err != nil {
		failMsg := fmt.Sprintf("LLM service error: %v", err)
		_ = UpdateMessageStatus(assistantMsg.ID, StatusFailed, nil, nil, &failMsg)
		return fmt.Errorf("%s", failMsg)
	}

	// 6. Update assistant message with response
	if err := UpdateMessageStatus(assistantMsg.ID, StatusCompleted, &responseContent, &tokensUsed, nil); err != nil {
		return fmt.Errorf("failed to update assistant message: %w", err)
	}

	log.Printf("Chat %s processed successfully", msg.ChatID)
	return nil
}

func buildPromptFromHistory(messages []Message) string {
	var sb strings.Builder
	// Simple chat template:
	// User: ...
	// Assistant: ...
	for _, m := range messages {
		// Skip the pending assistant message we just created (status pending, empty content)
		if m.Status == StatusPending && m.Role == RoleAssistant {
			continue
		}

		roleName := "User"
		switch m.Role {
		case RoleAssistant:
			roleName = "Assistant"
		case RoleSystem:
			roleName = "System"
		}

		sb.WriteString(fmt.Sprintf("%s: %s\n", roleName, m.Content))
	}
	// Append "Assistant: " to prompt the model to complete
	sb.WriteString("Assistant: ")
	return sb.String()
}

// callLLMService calls the llama.cpp server API
func callLLMService(baseURL, prompt string) (string, int, error) {
	reqBody := LlamaCppRequest{
		Prompt:    prompt,
		NPredict:  512,
		Stream:    false,
		MaxTokens: 2000,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 120 * time.Second, // LLM inference can take a while
	}

	req, err := http.NewRequest("POST", baseURL+"/completion", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("Calling LLM service at %s", baseURL)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to call LLM service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("LLM service returned status %d: %s", resp.StatusCode, string(body))
	}

	var llmResp LlamaCppResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode LLM response: %w", err)
	}

	tokensUsed := llmResp.TokensEvaluated + llmResp.TokensPredicted

	return llmResp.Content, tokensUsed, nil
}
