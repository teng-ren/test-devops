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

	//Routes to Authentication service
	mux.Handle("/auth/", http.StripPrefix("/auth", reverseProxy(auth_url)))

	//Routes to Admin service
	mux.Handle("/admin/", http.StripPrefix("/admin", reverseProxy(auth_url)))

	log.Println("API Gateway running on :8000")
	log.Fatal(http.ListenAndServe(":8000", corsMiddleware(mux)))
}
