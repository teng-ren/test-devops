package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
)

func main() {
	log.Println("Starting Prompt Manager service...")

	// Initialize components
	initDB()
	defer db.Close()

	initAuth()

	if err := initPubSub(); err != nil {
		log.Fatalf("Failed to initialize Pub/Sub: %v", err)
	}
	defer closePubSub()

	initWorker()
	startWorker()
	defer stopWorker()

	// Set up HTTP routes
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/models", modelsHandler)

	// Protected endpoints (auth required)
	// Protected endpoints (auth required)
	mux.HandleFunc("/chats", authMiddleware(chatsHandler))
	mux.HandleFunc("/chats/", authMiddleware(chatsHandler))

	// Get port from environment or default to 8002
	port := os.Getenv("PORT")
	if port == "" {
		port = "8002"
	}

	// Set up graceful shutdown
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		server.Close()
	}()

	log.Printf("Prompt Manager service running on :%s", port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}
