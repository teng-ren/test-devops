package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Request metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)

	// Auth metrics
	authFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_failures_total",
			Help: "Total number of authentication failures",
		},
		[]string{"reason"},
	)

	// Message queue metrics
	messagesInQueue = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "messages_in_queue",
			Help: "Current number of messages in the queue",
		},
	)

	messagesProcessedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "messages_processed_total",
			Help: "Total number of messages processed",
		},
	)

	// User metrics
	activeUsersTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_users_total",
			Help: "Current number of active users",
		},
	)

	userRegistrationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "user_registrations_total",
			Help: "Total number of user registrations",
		},
	)

	// Security event metrics
	securityEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "security_events_total",
			Help: "Total number of security events",
		},
		[]string{"event_type", "severity"},
	)

	// Uptime metric
	serviceUptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "service_uptime_seconds",
			Help: "Service uptime in seconds",
		},
	)

	startTime = time.Now()
)

type MetricsCollector struct {
	db *sql.DB
}

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(authFailuresTotal)
	prometheus.MustRegister(messagesInQueue)
	prometheus.MustRegister(messagesProcessedTotal)
	prometheus.MustRegister(activeUsersTotal)
	prometheus.MustRegister(userRegistrationsTotal)
	prometheus.MustRegister(securityEventsTotal)
	prometheus.MustRegister(serviceUptime)
}

func NewMetricsCollector() (*MetricsCollector, error) {
	authHost := getEnv("POSTGRES_HOST", "auth-postgres-service")
	authPort := getEnv("POSTGRES_PORT", "5432")
	authDB := getEnv("POSTGRES_DB", "app_db")
	authUser := getEnv("POSTGRES_USER", "app_user")
	authPass := getEnv("POSTGRES_PASSWORD", "app_password")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		authHost, authPort, authUser, authPass, authDB)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &MetricsCollector{db: db}, nil
}

func (mc *MetricsCollector) CollectAuthMetrics(ctx context.Context) error {
	// Count auth failures from logs or events table
	var failureCount int64
	query := `
		SELECT COUNT(*) FROM auth_events 
		WHERE event_type = 'auth_failure' 
		AND created_at > NOW() - INTERVAL '5 minutes'
	`
	err := mc.db.QueryRowContext(ctx, query).Scan(&failureCount)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error collecting auth metrics: %v", err)
	} else {
		authFailuresTotal.WithLabelValues("invalid_credentials").Add(float64(failureCount))
	}

	return nil
}

func (mc *MetricsCollector) CollectUserMetrics(ctx context.Context) error {
	// Count active users (users who logged in within last 24 hours)
	var activeCount int64
	query := `
		SELECT COUNT(DISTINCT user_id) FROM auth_events 
		WHERE event_type = 'login_success' 
		AND created_at > NOW() - INTERVAL '24 hours'
	`
	err := mc.db.QueryRowContext(ctx, query).Scan(&activeCount)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error collecting user metrics: %v", err)
	} else {
		activeUsersTotal.Set(float64(activeCount))
	}

	// Count total users
	var totalUsers int64
	err = mc.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error counting users: %v", err)
	}

	return nil
}

func (mc *MetricsCollector) CollectSecurityMetrics(ctx context.Context) error {
	// Collect security events by type and severity
	query := `
		SELECT event_type, severity, COUNT(*) 
		FROM security_events 
		WHERE created_at > NOW() - INTERVAL '5 minutes'
		GROUP BY event_type, severity
	`
	rows, err := mc.db.QueryContext(ctx, query)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error collecting security metrics: %v", err)
		return nil
	}
	if rows != nil {
		defer rows.Close()

		for rows.Next() {
			var eventType, severity string
			var count int64
			if err := rows.Scan(&eventType, &severity, &count); err != nil {
				continue
			}
			securityEventsTotal.WithLabelValues(eventType, severity).Add(float64(count))
		}
	}

	return nil
}

func (mc *MetricsCollector) StartCollecting() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			mc.CollectAuthMetrics(ctx)
			mc.CollectUserMetrics(ctx)
			mc.CollectSecurityMetrics(ctx)

			// Update uptime
			serviceUptime.Set(time.Since(startTime).Seconds())
			cancel()
		}
	}()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"uptime": time.Since(startTime).String(),
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	log.Println("Starting metrics exporter...")

	collector, err := NewMetricsCollector()
	if err != nil {
		log.Printf("Warning: Could not connect to database: %v", err)
		log.Println("Continuing without database metrics...")
	} else {
		collector.StartCollecting()
		log.Println("Database metrics collector started")
	}

	// Simulate some metrics for testing
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			// Simulate request metrics
			httpRequestsTotal.WithLabelValues("api-gateway", "GET", "200").Inc()
			httpRequestsTotal.WithLabelValues("auth-service", "POST", "200").Inc()

			// Simulate queue depth (mock)
			messagesInQueue.Set(float64(time.Now().Unix() % 100))
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", healthHandler)

	port := getEnv("PORT", "9090")
	log.Printf("Metrics exporter listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
