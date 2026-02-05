package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Available models
var availableModels = []string{"gemma3", "qwen3"}

// healthHandler returns service health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := HealthResponse{
		Status:  "healthy",
		Service: "prompt-manager",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// modelsHandler returns available models
func modelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := APIResponse{
		Data: GetModelsData{
			Models: availableModels,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// createChatHandler handles POST /chats
func createChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := getUserID(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Model == "" {
		req.Model = "gemma3" // Default model
	}

	// Validate model
	validModel := slices.Contains(availableModels, req.Model)
	if !validModel {
		http.Error(w, "Invalid model", http.StatusBadRequest)
		return
	}

	// Create chat in database
	chat, err := CreateChat(userID, req.Model)
	if err != nil {
		log.Printf("Error creating chat: %v", err)
		http.Error(w, "Failed to create chat", http.StatusInternalServerError)
		return
	}

	resp := APIResponse{
		Data: map[string]interface{}{
			"chat": chat,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// listChatsHandler handles GET /chats
func listChatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := getUserID(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse pagination parameters
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	chats, totalCount, err := ListChats(userID, page, pageSize)
	if err != nil {
		log.Printf("Error listing chats: %v", err)
		http.Error(w, "Failed to list chats", http.StatusInternalServerError)
		return
	}

	if chats == nil {
		chats = []Chat{}
	}

	resp := ChatListResponse{
		Chats:      chats,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getChatHandler handles GET /chats/{id}
func getChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := getUserID(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := strings.TrimPrefix(r.URL.Path, "/chats/")
	if chatID == "" {
		http.Error(w, "Chat ID is required", http.StatusBadRequest)
		return
	}

	chat, err := GetChat(chatID, userID)
	if err != nil {
		log.Printf("Error getting chat: %v", err)
		http.Error(w, "Failed to get chat", http.StatusInternalServerError)
		return
	}
	if chat == nil {
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}

	messages, err := GetMessages(chatID)
	if err != nil {
		log.Printf("Error getting messages: %v", err)
		http.Error(w, "Failed to get messages", http.StatusInternalServerError)
		return
	}
	if messages == nil {
		messages = []Message{}
	}

	resp := ChatWithMessages{
		Chat:     *chat,
		Messages: messages,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// deleteChatHandler handles DELETE /chats/{id}
func deleteChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := getUserID(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := strings.TrimPrefix(r.URL.Path, "/chats/")
	if chatID == "" {
		http.Error(w, "Chat ID is required", http.StatusBadRequest)
		return
	}

	err := DeleteChat(chatID, userID)
	if err == sql.ErrNoRows {
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Error deleting chat: %v", err)
		http.Error(w, "Failed to delete chat", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// sendMessageHandler handles POST /chats/{id}/messages
func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := getUserID(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract chat ID
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	chatID := pathParts[2] // /chats/{id}/messages...

	// Verify chat exists and belongs to user
	chat, err := GetChat(chatID, userID)
	if err != nil {
		log.Printf("Error checking chat: %v", err)
		http.Error(w, "Failed to check chat", http.StatusInternalServerError)
		return
	}
	if chat == nil {
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	// Create user message
	userMsg, err := CreateMessage(chatID, RoleUser, req.Content)
	if err != nil {
		log.Printf("Error creating message: %v", err)
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	// Publish to Pub/Sub
	msg := PubSubMessage{
		ChatID:    chatID,
		MessageID: userMsg.ID,
		UserID:    userID,
		Model:     chat.Model,
	}
	if err := publishMessage(msg); err != nil {
		log.Printf("Error publishing message: %v", err)
		// We still return success as the message is saved
	}

	resp := APIResponse{
		Data: map[string]interface{}{
			"message": userMsg,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// chatsHandler routes requests to the appropriate handler
func chatsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request for %s %s", r.Method, r.URL.Path)
	path := r.URL.Path

	// /chats or /chats/
	if path == "/chats" || path == "/chats/" {
		switch r.Method {
		case http.MethodGet:
			listChatsHandler(w, r)
		case http.MethodPost:
			createChatHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /chats/{id}/messages
	if strings.Contains(path, "/messages") {
		sendMessageHandler(w, r)
		return
	}

	// /chats/{id}
	if strings.HasPrefix(path, "/chats/") {
		switch r.Method {
		case http.MethodGet:
			getChatHandler(w, r)
		case http.MethodDelete:
			deleteChatHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.NotFound(w, r)
}
