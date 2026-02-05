package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func reverseProxy(target string) http.Handler {
	url, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(url)
	return proxy
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()
	auth_url := os.Getenv("AUTH_SERVICE_URL")
	prompt_manager_url := os.Getenv("PROMPT_MANAGER_SERVICE_URL")

	//Routes to Authentication service
	mux.Handle("/auth/", http.StripPrefix("/auth", reverseProxy(auth_url)))

	//Routes to Admin service
	mux.Handle("/admin/", http.StripPrefix("/admin", reverseProxy(auth_url)))

	//Routes to Prompt Manager service
	mux.Handle("/chats/", http.StripPrefix("/chats", reverseProxy(prompt_manager_url+"/chats")))
	mux.Handle("/chats", reverseProxy(prompt_manager_url))
	mux.Handle("/models", reverseProxy(prompt_manager_url))

	// Get port from environment or default to 8000
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	log.Println("API Gateway running on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(mux)))
}
