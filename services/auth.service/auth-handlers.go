package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)



var db *sql.DB
var jwtSecret []byte

func initDB() {
	var err error
	connStr := "host=" + os.Getenv("DB_HOST") +
		" port=" + os.Getenv("DB_PORT") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" sslmode=disable"

	log.Println("Connecting to database with:", connStr)

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("JWT_SECRET is not set")
	}

	log.Println("Database connected successfully")
}

func getUserByUsername(username string) (*User_Auth, error) {
	var user User_Auth
	err := db.QueryRow("SELECT id, username, password_hash, role FROM users WHERE username = $1", username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role)
	
	if err != nil {
		log.Printf("Error fetching user %s: %v", username, err)
		return nil, err
	}

	log.Printf("Found user: %s, role: %s, hash length: %d", user.Username, user.Role, len(user.PasswordHash))
	return &user, nil
}

func checkPassword(hashedPassword, password string) bool {
	log.Printf("Checking password - Hash length: %d, Password length: %d", len(hashedPassword), len(password))
	
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	
	if err != nil {
		log.Printf("Password check failed: %v", err)
		return false
	}
	
	log.Println("Password check successful")
	return true
}

func generateJWT(userID int, username, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func validateJWT(tokenString string) (bool, string) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		log.Printf("Token validation failed: %v", err)
		return false, ""
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if role, exists := claims["role"]; exists {
			if roleStr, ok := role.(string); ok {
				return true, roleStr
			}
		}
	}

	return true, ""
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Failed to decode request: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	
	log.Printf("Login attempt for user: %s", req.Username)

	user, err := getUserByUsername(req.Username)
	if err != nil {
		log.Printf("User not found: %s", req.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !checkPassword(user.PasswordHash, req.Password) {
		log.Printf("Password mismatch for user: %s", req.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := generateJWT(user.ID, user.Username, user.Role)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	log.Printf("Login successful for user: %s", req.Username)
	resp := LoginResponse{Token: token}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	valid, role := validateJWT(req.Token)
	resp := ValidateResponse{Valid: valid, Role: role}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

