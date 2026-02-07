package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "status"},
	)
	authFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_failures_total",
			Help: "Total number of authentication failures",
		},
	)
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
	mux.Handle("/metrics", promhttp.Handler())

	//Authentication
	mux.HandleFunc("/login", metricsMiddleware(loginHandler))
	//Authorization
	mux.HandleFunc("/validate", metricsMiddleware(validateHandler))

	mux.HandleFunc("GET /users", metricsMiddleware(authMiddleware(getUsersHandler)))
	mux.HandleFunc("POST /users", metricsMiddleware(authMiddleware(createUserHandler)))
	mux.HandleFunc("PUT /users/", metricsMiddleware(authMiddleware(editUserHandler)))
	mux.HandleFunc("DELETE /users/", metricsMiddleware(authMiddleware(deleteUserHandler)))

	// Get port from environment or default to 8001
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	log.Println("Auth service running on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func metricsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Track request
		httpRequestsTotal.WithLabelValues("auth-service", r.Method, "200").Inc()
		next(w, r)
	}
}
