package main

import (
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
)

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type User_Auth struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"passwordhash"`
	Role         string `json:"role"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdateUserRequest struct {
	Role string `json:"role"`
}

type ValidateRequest struct {
	Token string `json:"token"`
}

type ValidateResponse struct {
	Valid bool   `json:"valid"`
	Role  string `json:"role"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	//Authentication
	mux.HandleFunc("/login", loginHandler)
	//Authorization
	mux.HandleFunc("/validate", validateHandler)

	mux.HandleFunc("GET /users", authMiddleware(getUsersHandler))
	mux.HandleFunc("POST /users", authMiddleware(createUserHandler))
	mux.HandleFunc("PUT /users/", authMiddleware(editUserHandler))
	mux.HandleFunc("DELETE /users/", authMiddleware(deleteUserHandler))

	log.Println("Auth service running on :8001")
	log.Fatal(http.ListenAndServe(":8001", mux))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
