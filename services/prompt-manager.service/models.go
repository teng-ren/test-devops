package main

import (
	"time"
)

// MessageRole represents the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// MessageStatus represents the state of a message
type MessageStatus string

const (
	StatusPending   MessageStatus = "pending"
	StatusStreaming MessageStatus = "streaming"
	StatusCompleted MessageStatus = "completed"
	StatusFailed    MessageStatus = "failed"
)

// Chat represents a chat conversation
type Chat struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	Title     string    `json:"title"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a message in a chat
type Message struct {
	ID           string        `json:"id"`
	ChatID       string        `json:"chat_id"`
	Role         MessageRole   `json:"role"`
	Content      string        `json:"content"`
	Status       MessageStatus `json:"status"`
	ErrorMessage *string       `json:"error_message,omitempty"`
	TokensUsed   *int          `json:"tokens_used,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// CreateChatRequest is the request body for creating a new chat
type CreateChatRequest struct {
	Model string `json:"model"`
}

// SendMessageRequest is the request body for sending a message
type SendMessageRequest struct {
	Content string `json:"content"`
}

// ChatWithMessages includes the chat details and its messages
type ChatWithMessages struct {
	Chat     Chat      `json:"chat"`
	Messages []Message `json:"messages"`
}

// ChatListResponse is returned for listing chats
type ChatListResponse struct {
	Chats      []Chat `json:"chats"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalCount int    `json:"total_count"`
}

// MessageListResponse is returned for listing messages
type MessageListResponse struct {
	Messages   []Message `json:"messages"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalCount int       `json:"total_count"`
}

// PubSubMessage is the message format for Pub/Sub
type PubSubMessage struct {
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"` // The user message ID that triggered this
	UserID    int    `json:"user_id"`
	Model     string `json:"model"`
}

// AuthValidateRequest is sent to auth service for token validation
type AuthValidateRequest struct {
	Token string `json:"token"`
}

// AuthValidateResponse is received from auth service
type AuthValidateResponse struct {
	Valid    bool   `json:"valid"`
	Role     string `json:"role"`
	UserID   int    `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
}

// GetModelsData contains the list of available models
type GetModelsData struct {
	Models []string `json:"models"`
}

// APIResponse is a generic API response wrapper
type APIResponse struct {
	Data any `json:"data"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse is returned by the health endpoint
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}
