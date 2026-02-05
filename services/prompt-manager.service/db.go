package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

func initDB() {
	var err error
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	log.Println("Connecting to database...")

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("Database connected successfully")
}

// CreateChat creates a new chat session
func CreateChat(userID int, model string) (*Chat, error) {
	chat := &Chat{
		UserID: userID,
		Model:  model,
		Title:  "New Chat", // Default title
	}

	err := db.QueryRow(
		`INSERT INTO chats (user_id, model, title)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		userID, model, chat.Title,
	).Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)

	if err != nil {
		log.Printf("Error creating chat: %v", err)
		return nil, err
	}

	return chat, nil
}

// GetChat retrieves a chat by ID
func GetChat(id string, userID int) (*Chat, error) {
	chat := &Chat{}
	err := db.QueryRow(
		`SELECT id, user_id, title, model, created_at, updated_at
		 FROM chats WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(
		&chat.ID, &chat.UserID, &chat.Title, &chat.Model,
		&chat.CreatedAt, &chat.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		log.Printf("Error getting chat: %v", err)
		return nil, err
	}

	return chat, nil
}

// ListChats retrieves chats for a user with pagination
func ListChats(userID, page, pageSize int) ([]Chat, int, error) {
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM chats WHERE user_id = $1`,
		userID,
	).Scan(&totalCount)
	if err != nil {
		log.Printf("Error counting chats: %v", err)
		return nil, 0, err
	}

	// Get chats
	rows, err := db.Query(
		`SELECT id, user_id, title, model, created_at, updated_at
		 FROM chats
		 WHERE user_id = $1
		 ORDER BY updated_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, pageSize, offset,
	)
	if err != nil {
		log.Printf("Error listing chats: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		err := rows.Scan(
			&c.ID, &c.UserID, &c.Title, &c.Model,
			&c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error scanning chat: %v", err)
			return nil, 0, err
		}
		chats = append(chats, c)
	}

	return chats, totalCount, nil
}

// DeleteChat deletes a chat and its messages
func DeleteChat(id string, userID int) error {
	result, err := db.Exec(
		`DELETE FROM chats WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		log.Printf("Error deleting chat: %v", err)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// CreateMessage adds a message to a chat
func CreateMessage(chatID string, role MessageRole, content string) (*Message, error) {
	msg := &Message{
		ChatID:  chatID,
		Role:    role,
		Content: content,
		Status:  StatusCompleted, // Default
	}

	if role == RoleAssistant {
		msg.Status = StatusPending
	}

	err := db.QueryRow(
		`INSERT INTO messages (chat_id, role, content, status)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		chatID, role, content, msg.Status,
	).Scan(&msg.ID, &msg.CreatedAt)

	if err != nil {
		log.Printf("Error creating message: %v", err)
		return nil, err
	}

	// Update chat's updated_at timestamp
	_, _ = db.Exec(`UPDATE chats SET updated_at = CURRENT_TIMESTAMP WHERE id = $1`, chatID)

	return msg, nil
}

// GetMessages retrieves messages for a chat
func GetMessages(chatID string) ([]Message, error) {
	rows, err := db.Query(
		`SELECT id, chat_id, role, content, status, error_message, tokens_used, created_at
		 FROM messages
		 WHERE chat_id = $1
		 ORDER BY created_at ASC`,
		chatID,
	)
	if err != nil {
		log.Printf("Error getting messages: %v", err)
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		err := rows.Scan(
			&m.ID, &m.ChatID, &m.Role, &m.Content, &m.Status,
			&m.ErrorMessage, &m.TokensUsed, &m.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning message: %v", err)
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, nil
}

// UpdateMessageStatus updates the status of a message
func UpdateMessageStatus(id string, status MessageStatus, content *string, tokens *int, errorMsg *string) error {
	query := `UPDATE messages SET status = $1, updated_at = CURRENT_TIMESTAMP`
	args := []interface{}{status}
	argIdx := 2

	if content != nil {
		query += fmt.Sprintf(", content = $%d", argIdx)
		args = append(args, *content)
		argIdx++
	}
	if tokens != nil {
		query += fmt.Sprintf(", tokens_used = $%d", argIdx)
		args = append(args, *tokens)
		argIdx++
	}
	if errorMsg != nil {
		query += fmt.Sprintf(", error_message = $%d", argIdx)
		args = append(args, *errorMsg)
		argIdx++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	_, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("Error updating message status: %v", err)
	}
	return err
}

// UpdateChatTitle updates the title of a chat
func UpdateChatTitle(id string, title string) error {
	_, err := db.Exec(
		`UPDATE chats SET title = $1 WHERE id = $2`,
		title, id,
	)
	return err
}
