package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type AlertmanagerPayload struct {
	Status string  `json:"status"`
	Alerts []Alert `json:"alerts"`
}

type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type DiscordMessage struct {
	Content  string         `json:"content,omitempty"`
	Embeds   []DiscordEmbed `json:"embeds"`
	Username string         `json:"username,omitempty"`
}

type DiscordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func main() {
	port := getEnv("PORT", "8080")
	critical := getEnv("DISCORD_CRITICAL_URLS", "")
	warning := getEnv("DISCORD_WARNING_URLS", "")
	main := getEnv("DISCORD_MAIN_URLS", "")
	phone := getEnv("DISCORD_PHONE_URL", "")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/webhook/critical", makeHandler(critical))
	mux.HandleFunc("/webhook/warning", makeHandler(warning))
	mux.HandleFunc("/webhook/main", makeHandler(main))
	mux.HandleFunc("/webhook/phone", makeHandler(phone))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("discord-relay listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func makeHandler(urlsEnv string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		urls := splitCSV(urlsEnv)
		if len(urls) == 0 {
			http.Error(w, "no discord webhook urls configured", http.StatusInternalServerError)
			return
		}

		var payload AlertmanagerPayload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "invalid json payload", http.StatusBadRequest)
			return
		}

		if len(payload.Alerts) == 0 {
			http.Error(w, "no alerts in payload", http.StatusBadRequest)
			return
		}

		alertJSON, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "marshal alerts: "+err.Error(), http.StatusInternalServerError)
			return
		}

		for _, url := range urls {
			if err := postDiscord(url, string(alertJSON)); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}

func queryPrometheus(query string) (float64, error) {
	prometheusURL := getEnv("PROMETHEUS_URL", "http://prometheus:9090")
	url := fmt.Sprintf("%s/api/v1/query?query=%s", prometheusURL, query)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Data.Result) == 0 {
		return 0, nil
	}

	if len(result.Data.Result[0].Value) < 2 {
		return 0, fmt.Errorf("invalid response format")
	}

	valueStr := fmt.Sprintf("%v", result.Data.Result[0].Value[1])
	var value float64
	if _, err := fmt.Sscanf(valueStr, "%f", &value); err != nil {
		return 0, fmt.Errorf("failed to parse value: %w", err)
	}
	return value, nil
}

func enrichHeartbeatAlert(alert *Alert) {
	if alert.Labels["alertname"] != "SystemHeartbeat" {
		return
	}

	metrics := []struct {
		name  string
		query string
		unit  string
	}{
		{"CPU Usage", "avg(node_cpu_usage_percent)", "%"},
		{"Memory Usage", "avg(node_memory_usage_percent)", "%"},
		{"API RPS", "sum(api_requests_per_second)", "req/s"},
		{"Error Rate", "avg(api_error_rate)*100", "%"},
		{"Prompts Blocked", "sum(prompts_blocked_recent)", ""},
		{"Nodes Ready", "sum(nodes_ready_total)", ""},
		{"Pods Running", "sum(pods_running_total)", ""},
	}

	var description strings.Builder
	description.WriteString("System is healthy:\n")

	for _, m := range metrics {
		value, err := queryPrometheus(m.query)
		if err != nil {
			log.Printf("Failed to query %s: %v", m.name, err)
			continue
		}
		if m.unit != "" {
			description.WriteString(fmt.Sprintf("• %s: %.2f%s\n", m.name, value, m.unit))
		} else {
			description.WriteString(fmt.Sprintf("• %s: %.0f\n", m.name, value))
		}
	}

	alert.Annotations["description"] = description.String()
}

func buildEmbeds(payload AlertmanagerPayload) []DiscordEmbed {
	var embeds []DiscordEmbed
	status := firstNonEmpty(payload.Status, "firing")
	isResolved := status == "resolved"

	maxAlerts := 10
	alertCount := len(payload.Alerts)
	if alertCount > maxAlerts {
		alertCount = maxAlerts
	}

	for i := 0; i < alertCount; i++ {
		alert := payload.Alerts[i]

		// Enrich heartbeat alerts with live metrics
		enrichHeartbeatAlert(&alert)

		name := firstNonEmpty(alert.Labels["alertname"], "unknown")
		severity := firstNonEmpty(alert.Labels["severity"], "warning")
		summary := firstNonEmpty(alert.Annotations["summary"], alert.Annotations["description"])
		description := firstNonEmpty(alert.Annotations["description"])
		instance := firstNonEmpty(alert.Labels["instance"], alert.Labels["pod"], alert.Labels["service"])
		category := firstNonEmpty(alert.Labels["category"], "general")

		color := severityToColor(severity, isResolved)
		emoji := getEmoji(severity, isResolved)

		titleStatus := strings.ToUpper(severity)
		if isResolved {
			titleStatus = "RESOLVED"
		}

		embed := DiscordEmbed{
			Title:     fmt.Sprintf("%s %s [%s]", emoji, name, titleStatus),
			Color:     color,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}

		if summary != "" {
			embed.Description = summary
		}

		var fields []DiscordEmbedField

		if category != "" && category != "general" {
			fields = append(fields, DiscordEmbedField{
				Name:   "Category",
				Value:  strings.Title(category),
				Inline: true,
			})
		}

		if severity != "" {
			fields = append(fields, DiscordEmbedField{
				Name:   "Severity",
				Value:  strings.ToUpper(severity),
				Inline: true,
			})
		}

		if description != "" && description != summary {
			fields = append(fields, DiscordEmbedField{
				Name:   "Details",
				Value:  description,
				Inline: false,
			})
		}

		if instance != "" {
			fields = append(fields, DiscordEmbedField{
				Name:   "Instance",
				Value:  instance,
				Inline: true,
			})
		}

		embed.Fields = fields
		embed.Footer = &DiscordEmbedFooter{
			Text: "Alertmanager - Monitoring System",
		}

		embeds = append(embeds, embed)
	}

	if len(payload.Alerts) > maxAlerts {
		embed := DiscordEmbed{
			Title:       "⚠️ More Alerts",
			Color:       0xFFA500,
			Description: fmt.Sprintf("Showing %d of %d alerts. %d more not displayed.", maxAlerts, len(payload.Alerts), len(payload.Alerts)-maxAlerts),
		}
		embeds = append(embeds, embed)
	}

	return embeds
}

func severityToColor(severity string, isResolved bool) int {
	if isResolved {
		return 0x27AE60
	}
	switch strings.ToLower(severity) {
	case "critical":
		return 0xD32F2F
	case "warning":
		return 0xFFA500
	case "info":
		return 0x0099FF
	default:
		return 0x808080
	}
}

func getEmoji(severity string, isResolved bool) string {
	if isResolved {
		return "✅"
	}
	switch strings.ToLower(severity) {
	case "critical":
		return "🔴"
	case "warning":
		return "🟡"
	case "info":
		return "ℹ️"
	default:
		return "⚪"
	}
}

func postDiscord(url, message string) error {
	// Check if this is a pushcall.me phone call URL
	if strings.Contains(url, "pushcall.me") {
		return makePhoneCall(url)
	}

	// Handle Discord webhooks
	var alerts AlertmanagerPayload
	err := json.Unmarshal([]byte(message), &alerts)
	if err != nil {
		return fmt.Errorf("unmarshal alerts: %w", err)
	}

	embeds := buildEmbeds(alerts)
	payload := DiscordMessage{
		Embeds:   embeds,
		Username: "Monitoring Alert",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord message: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned %s", resp.Status)
	}

	return nil
}

func makePhoneCall(url string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("phone call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("phone call API returned %s", resp.Status)
	}

	log.Printf("Phone call triggered successfully")
	return nil
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max-1] + "\u2026"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
