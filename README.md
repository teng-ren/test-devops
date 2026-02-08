# Monitoring Stack - Complete Guide

## Table of Contents
1. [Overview](#overview)
2. [Architecture Diagram](#architecture-diagram)
3. [Component Details](#component-details)
4. [Configuration Files](#configuration-files)
5. [Data Flow](#data-flow)
6. [Alert Routing](#alert-routing)
7. [Deployment](#deployment)
8. [Troubleshooting](#troubleshooting)

---

## Overview

This monitoring folder contains a complete observability stack for Kubernetes applications deployed in Google Cloud Platform (GKE). The stack monitors infrastructure health, application performance, security events, and sends real-time alerts to Discord channels.

**Core Technologies:**
- **Prometheus**: Metrics collection and storage
- **Grafana**: Visualization and dashboards
- **Alertmanager**: Alert routing and management
- **Custom Exporters**: Application-specific metrics
- **Discord Integration**: Real-time notifications

**What It Monitors:**
- ✅ Kubernetes cluster health (nodes, pods, containers)
- ✅ Application metrics (HTTP requests, latency, errors)
- ✅ Security events (authentication failures, unauthorized access)
- ✅ Resource utilization (CPU, memory, disk, network)
- ✅ Database metrics (auth failures, active users)
- ✅ GCP-specific metrics (GKE cluster metrics)

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     KUBERNETES CLUSTER (GKE)                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐        ┌──────────────┐                       │
│  │ Application  │  ◄───► │   Metrics    │                       │
│  │   Services   │  HTTP  │  Exporter    │────┐                  │
│  │              │        └──────────────┘    │                  │
│  │- API Gateway │                             │                  │
│  │- Auth        │        ┌──────────────┐    │   Scrapes        │
│  │- Prompt Mgr  │  ◄───► │ GCP Exporter │────┤  Metrics         │
│  │- LLM         │  GCP   └──────────────┘    │  (Pull)          │
│  └──────────────┘        Monitoring API       │                  │
│         │                                      ▼                  │
│         │                            ┌─────────────────┐         │
│         │                            │   PROMETHEUS    │         │
│         │                            │   (Port 9090)   │         │
│         │                            └─────────────────┘         │
│         │                                     │                  │
│         │                                     │ Evaluates        │
│         │                                     │ Rules            │
│  ┌──────────────┐                            ▼                  │
│  │Node Exporter │───────────────┐   ┌─────────────────┐        │
│  │(DaemonSet)   │               │   │  ALERTMANAGER   │        │
│  └──────────────┘               │   │   (Port 9093)   │        │
│  Host Metrics                   │   └─────────────────┘        │
│                                  │            │                 │
│  ┌──────────────┐               │            │ Routes          │
│  │Kube-State    │───────────────┘            │ Alerts          │
│  │Metrics       │                            ▼                 │
│  └──────────────┘                   ┌─────────────────┐       │
│  K8s Object States                  │ Discord Relay   │       │
│                                      │  (Port 8080)    │       │
│                                      └─────────────────┘       │
│                                               │                 │
└───────────────────────────────────────────────┼─────────────────┘
                                                │
                                                │ HTTP POST
                                                ▼
                                    ┌──────────────────────┐
                                    │  DISCORD WEBHOOKS   │
                                    ├──────────────────────┤
                                    │ 🔴 Critical Channel  │
                                    │ 🟡 Warning Channel   │
                                    │ ℹ️  Main Channel     │
                                    └──────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                         VISUALIZATION                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│                    ┌─────────────────┐                           │
│                    │    GRAFANA      │                           │
│                    │   (Port 3000)   │                           │
│                    └─────────────────┘                           │
│                            │                                     │
│                            │ Queries                             │
│                            ▼                                     │
│                    ┌─────────────────┐                           │
│                    │   Prometheus    │                           │
│                    └─────────────────┘                           │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Component Details

### 1. Prometheus (`prometheus/`)

**Purpose**: The heart of the monitoring system. Collects, stores, and queries time-series metrics data.

#### Files:
- **`configmap.yaml`**: Main configuration file (282 lines)
- **`deployment.yaml`**: Kubernetes deployment specification
- **`services.yaml`**: Service definitions for networking
- **`serviceaccounts.yaml`**: RBAC permissions for cluster access
- **`pvc.yaml`**: Persistent storage for metrics data

#### 📚 Learning Focus: How Prometheus Works

**Core Concepts**:
1. **Pull Model**: Prometheus actively "scrapes" (fetches) metrics from targets every 15 seconds
2. **Service Discovery**: Automatically finds services/pods to monitor using Kubernetes API
3. **Time-Series Database**: Stores metrics with timestamps, allowing historical queries
4. **PromQL**: Query language to aggregate, filter, and analyze metrics
5. **TSDB Retention**: Keeps data for 7 days then automatically deletes old data

#### Key Configuration (`configmap.yaml`):

```yaml
# Global scrape settings
scrape_interval: 15s      # Collect metrics every 15 seconds
evaluation_interval: 15s  # Evaluate alerting rules every 15 seconds
```

**Scrape Configurations** (What Prometheus monitors):

#### 🔍 Code Deep Dive: Service Discovery

**1. `kubernetes-apiservers`** - Monitors Kubernetes Control Plane
```yaml
- job_name: 'kubernetes-apiservers'
  kubernetes_sd_configs:
    - role: endpoints           # Discover all endpoints in cluster
  scheme: https                 # Use HTTPS (not HTTP)
  tls_config:
    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  relabel_configs:
    # Only keep endpoints matching: namespace=default, service=kubernetes, port=https
    - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
      action: keep
      regex: default;kubernetes;https
```
**What this does**: Finds the Kubernetes API server endpoint and scrapes its metrics using the pod's service account token for authentication.

**2. `kubernetes-nodes`** - VM/Host Level Metrics
```yaml
- job_name: 'kubernetes-nodes'
  kubernetes_sd_configs:
    - role: node                # Discover all nodes
  relabel_configs:
    - action: labelmap         # Convert node labels to metric labels
      regex: __meta_kubernetes_node_label_(.+)
    - target_label: __address__
      replacement: kubernetes.default.svc:443
    - source_labels: [__meta_kubernetes_node_name]
      regex: (.+)
      target_label: __metrics_path__
      replacement: /api/v1/nodes/${1}/proxy/metrics
```
**What this does**: Instead of scraping nodes directly, it queries Kubernetes API which proxies the request to kubelet on each node. The path becomes `/api/v1/nodes/node-1/proxy/metrics`.

**3. `kubernetes-service-endpoints`** - Auto-discover Services
```yaml
- job_name: 'kubernetes-service-endpoints'
  relabel_configs:
    # Only scrape services with this annotation
    - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
      action: keep
      regex: true
    
    # Use custom port from annotation (if specified)
    - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
      action: replace
      target_label: __address__
      regex: ([^:]+)(?::\d+)?;(\d+)
      replacement: $1:$2        # Change from pod-ip:default-port to pod-ip:custom-port
```
**What this does**: Scans all services in cluster. If service has annotation `prometheus.io/scrape: "true"`, Prometheus adds it to scrape targets. If `prometheus.io/port: "9090"` exists, uses that port instead of default.

**4. `kubernetes-pods`** - Auto-discover Pods
```yaml
- job_name: 'kubernetes-pods'
  kubernetes_sd_configs:
    - role: pod                # Discover all pods
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
      action: keep
      regex: true
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
      action: replace
      target_label: __metrics_path__
      regex: (.+)               # Use custom path like /custom/metrics if specified
```
**What this does**: Same as services, but for individual pods. Useful when you want to scrape a specific pod without creating a service.

**5. `gcp-monitoring`** - GCP Cloud Metrics
```yaml
- job_name: 'gcp-monitoring'
  static_configs:
    - targets: ['gcp-exporter:8888']  # Static, not auto-discovered
  scrape_interval: 30s                # Scrape less frequently (GCP has rate limits)
  relabel_configs:
    - target_label: cluster
      replacement: 'ai-model-tester-2'
```
**What this does**: Points to our custom GCP exporter service which fetches data from GCP Cloud Monitoring API and converts it to Prometheus format.

#### 📊 Recording Rules - Pre-computed Metrics

**Why Recording Rules?**: Instead of calculating complex queries every time you view a dashboard, recording rules pre-compute them every 15 seconds and store the result as a new metric. This makes dashboards load faster and reduces CPU usage.

```yaml
# RULE 1: Requests Per Second (RPS)
- record: api_requests_per_second
  expr: sum(rate(http_requests_total[1m])) by (service, method, status)
```
**Breaking it down**:
- `http_requests_total` = Raw counter metric (increases forever)
- `rate(...[1m])` = Calculate per-second rate over last 1 minute
- `sum(...) by (service, method, status)` = Group by these labels
- **Result**: New metric `api_requests_per_second{service="api-gateway",method="GET",status="200"} 45.2`

**Example Query Result**:
```
api_requests_per_second{service="api-gateway",method="GET",status="200"} = 45.2
#### 🚨 Alerting Rules - When to Trigger Alerts

**Alert Anatomy**: Every alert has 4 parts:
1. **Condition** (`expr`) - The PromQL query that must be true
2. **Duration** (`for`) - How long condition must be true before firing
3. **Labels** - Metadata for routing (severity, category)
4. **Annotations** - Human-readable descriptions

```yaml
# ALERT 1: Security - Too Many Auth Failures
- alert: HighAuthFailureRate
  expr: sum(auth_failures) > 50
  for: 2m
  labels:
    severity: critical
    category: security
  annotations:
    summary: "High authentication failure rate detected"
    description: "{{ $value }} auth failures in the last 5 minutes"
```
**How it works**:
1. Every 15s, Prometheus evaluates: `sum(auth_failures) > 50`
2. If TRUE, alert enters "PENDING" state
3. If still TRUE after 2 minutes (`for: 2m`), alert "FIRES"
4. Alertmanager sees `severity: critical` → routes to critical channel
5. `{{ $value }}` = Actual number (e.g., "67 auth failures")

**Why 2 minutes?**: Prevents false alarms from temporary spikes.

```yaml
# ALERT 2: Performance - High Latency
- alert: HighLatency
  expr: api_request_latency_avg_ms > 1000
  for: 5m
  labels:
    severity: warning
    category: performance
  annotations:
    summary: "High API latency detected"
    description: "Average latency is {{ $value }}ms for {{ $labels.service }}"
```
**How it works**:
- Uses recording rule `api_request_latency_avg_ms` (computed every 15s)
- If latency > 1000ms (1 second) for 5 minutes straight → FIRE
- `{{ $labels.service }}` = Shows which service (e.g., "api-gateway")
- `severity: warning` → routes to QA team only

```yaml
# ALERT 3: Resources - High CPU
- alert: HighCPUUsage
  expr: pod_cpu_usage_percent > 80
  for: 10m
  labels:
    severity: warning
    category: resources
  annotations:
    summary: "High CPU usage on pod"
    description: "Pod {{ $labels.pod }} in {{ $labels.namespace }} is using {{ $value }}% CPU"
```
**How it works**:
- Checks every pod's CPU percentage
- If any pod > 80% for 10 minutes → FIRE
- Longer duration (10m) because CPU spikes are normal during deployments
- Shows specific pod name: "Pod prometheus-abc123 in dev is using 87% CPU"

```yaml
# ALERT 4: Heartbeat - System Health (Every 5 minutes)
- alert: SystemHeartbeat
  expr: label_replace(label_replace(avg(node_cpu_usage_percent), "metric", "node_cpu_avg", "", ""), "unit", "percent", "", "")
  for: 1m
  labels:
    severity: info
    category: heartbeat
  annotations:
    summary: "{{ $labels.metric }}: {{ $value }} {{ $labels.unit }}"
    description: "System is healthy - {{ $labels.metric }} = {{ $value }} {{ $labels.unit }}"
```
**How it works** (This is the most complex one):
- **Purpose**: Sends periodic "system is healthy" messages even when nothing is wrong
- Uses `label_replace()` to transform metrics into readable format
- Expression combines: CPU avg, memory avg, RPS, error rate, running pods, ready nodes
- `for: 1m` = Very short duration (fires quickly)
- Alertmanager configured with `repeat_interval: 5m` to send every 5 minutes
- **Result**: "node_cpu_avg: 45.2 percent" messages every 5 minutes

**Teacher Question**: "Why send alerts when nothing is wrong?"
**Answer**: Heartbeat proves the monitoring system itself is working. If you stop receiving heartbeats, you know the monitoring is down, not just that everything is perfect.*Latency**: Indicates performance issues
- **Error Rate**: Shows reliability problems

**Alerting Rules** (When to trigger alerts):

```yaml
# Security Alert: Too many auth failures
- alert: HighAuthFailureRate
  expr: sum(auth_failures) > 50
  for: 2m                        # Must persist for 2 minutes
  labels:
    severity: critical
    category: security
🔍 Code Deep Dive: Discord Relay

**HTTP Endpoints**:
- `/healthz`: Health check endpoint (returns "ok")
- `/webhook/critical`: Receives critical severity alerts
- `/webhook/warning`: Receives warning severity alerts
- `/webhook/main`: Receives info/heartbeat alerts

#### Code Walkthrough - `main.go`

**PART 1: Data Structures** (Lines 13-45)
```go
type AlertmanagerPayload struct {
    Status string  `json:"status"`      // "firing" or "resolved"
    Alerts []Alert `json:"alerts"`      // Array of alerts
}

type Alert struct {
    Labels      map[string]string `json:"labels"`      // severity, alertname, service
    Annotations map[string]string `json:"annotations"` // summary, description
}

type DiscordMessage struct {
    Embeds   []DiscordEmbed `json:"embeds"`   // Rich formatted messages
    Username string         `json:"username"`  // Bot name
}

type DiscordEmbed struct {
    Title       string              `json:"title"`       // "🔴 HighLatency [CRITICAL]"
    Description string              `json:"description"` // Summary text
    Color       int                 `json:"color"`       // Hex color (red=0xD32F2F)
    Fields      []DiscordEmbedField `json:"fields"`      // Key-value pairs
    Timestamp   string              `json:"timestamp"`   // ISO 8601 time
}
```
**What this does**: Defines the JSON structure for receiving data from Alertmanager and sending data to Discord. The `json:"..."` tags tell Go how to convert between JSON and Go structs.

**PART 2: Main Function** (Lines 48-72)
```go
func main() {
    // Read environment variables (set in deployment.yaml)
    port := getEnv("PORT", "8080")
    critical := getEnv("DISCORD_CRITICAL_URLS", "")    // Can be multiple URLs (CSV)
    warning := getEnv("DISCORD_WARNING_URLS", "")
**PART 4: Build Discord Embeds** (Lines 115-203)
```go
func buildEmbeds(payload AlertmanagerPayload) []DiscordEmbed {
    var embeds []DiscordEmbed
    
    // Check if alerts are resolved or firing
    status := payload.Status  // "firing" or "resolved"
    isResolved := status == "resolved"

    // Discord has message size limits - only show first 10 alerts
    maxAlerts := 10
    alertCount := len(payload.Alerts)
    if alertCount > maxAlerts {
        alertCount = maxAlerts
    }

    // Loop through each alert and create an embed
    for i := 0; i < alertCount; i++ {
        alert := payload.Alerts[i]
        
        // Extract data from alert (use firstNonEmpty to handle missing fields)
        name := firstNonEmpty(alert.Labels["alertname"], "unknown")
        severity := firstNonEmpty(alert.Labels["severity"], "warning")
        summary := firstNonEmpty(alert.Annotations["summary"], alert.Annotations["description"])
        description := firstNonEmpty(alert.Annotations["description"])
        instance := firstNonEmpty(alert.Labels["instance"], alert.Labels["pod"], alert.Labels["service"])
        category := firstNonEmpty(alert.Labels["category"], "general")

        // Choose color based on severity and status
        color := severityToColor(severity, isResolved)
        emoji := getEmoji(severity)

        // Create Discord embed
        embed := DiscordEmbed{
            Title:     fmt.Sprintf("%s %s [%s]", emoji, name, strings.ToUpper(severity)),
            Color:     color,
            Timestamp: time.Now().UTC().Format(time.RFC3339),
        }

        // Add summary as main description
        if summary != "" {
            embed.Description = summary
        }

        // Build fields (shown as columns in Discord)
        var fields []DiscordEmbedField
        
        if category != "" && category != "general" {
            fields = append(fields, DiscordEmbedField{
                Name:   "Category",
                Value:  strings.Title(category),  // "security" → "Security"
                Inline: true,                     // Show in column
            })
        }

        if instance != "" {
            fields = append(fields, DiscordEmbedField{
                Name:   "Instance",
                Value:  instance,                 // e.g., "api-gateway-abc123"
                Inline: true,
            })
        }

        embed.Fields = fields
        embed.Footer = &DiscordEmbedFooter{
            Text: "Alertmanager - Monitoring System",
        }

        embeds = append(embeds, embed)
    }

    // If more than 10 alerts, add a "..." embed
    if len(payload.Alerts) > maxAlerts {
        embed := DiscordEmbed{
            Title:       "⚠️ More Alerts",
            Color:       0xFFA500,  // Orange
            Description: fmt.Sprintf("Showing %d of %d alerts. %d more not displayed.", 
                                    maxAlerts, len(payload.Alerts), len(payload.Alerts)-maxAlerts),
        }
#### 🔍 Code Deep Dive: Database Metrics

**PART 1: Prometheus Metric Definitions** (Lines 18-98)
```go
// Counter: Increases forever (total requests since startup)
httpRequestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests",
    },
    []string{"service", "method", "status"},  // Labels for grouping
)

// Histogram: Tracks distribution of values (latency buckets)
httpRequestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request latencies in seconds",
        Buckets: prometheus.DefBuckets,  // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
    },
    []string{"service", "method"},
)

// Gauge: Can go up or down (current value)
authFailures = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "auth_failures",
        Help: "Recent authentication failures in the last 5 minutes",
    },
    []string{"reason"},
)
```
**Prometheus Metric Types Explained**:
1. **Counter**: Only goes up (resets to 0 on restart). Use for: requests, errors, events
2. **Gauge**: Goes up and down. Use for: temperature, CPU%, queue depth, active connections
3. **Histogram**: Buckets of observations. Use for: request duration, response size

**PART 2: Database Connection** (Lines 109-128)
```go
func NewMetricsCollector() (*MetricsCollector, error) {
    // Read connection details from environment variables
    authHost := getEnv("POSTGRES_HOST", "auth-postgres-service")
    authPort := getEnv("POSTGRES_PORT", "5432")
    authDB := getEnv("POSTGRES_DB", "app_db")
    authUser := getEnv("POSTGRES_USER", "app_user")
    authPass := getEnv("POSTGRES_PASSWORD", "app_password")

    // Build PostgreSQL connection string
    connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        authHost, authPort, authUser, authPass, authDB)

    // Open database connection
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }

    // Test connection (this is important! Open() doesn't actually connect)
    if err := db.Ping(); err != nil {
        return nil, err
    }

    return &MetricsCollector{db: db}, nil
}
```
**What this does**: Establishes connection to PostgreSQL database where application logs auth events. The `Ping()` call ensures database is actually reachable.

**PART 3: Query Database** (Lines 130-149)
```go
func (mc *MetricsCollector) CollectAuthMetrics(ctx context.Context) error {
    // Count auth failures in last 5 minutes
    var failureCount int64
    query := `
        SELECT COUNT(*) FROM auth_events 
        WHERE event_type = 'auth_failure' 
        AND created_at > NOW() - INTERVAL '5 minutes'
    `
    
    // Execute query (use QueryRowContext for single row result)
    err := mc.db.QueryRowContext(ctx, query).Scan(&failureCount)
    
    if err != nil && err != sql.ErrNoRows {
        log.Printf("Error collecting auth metrics: %v", err)
    } else {
        // Update Prometheus metric
        authFailures.WithLabelValues("invalid_credentials").Set(float64(failureCount))
    }

    return nil
}
```
**What this does**:
1. Runs SQL query to count auth failures in last 5 minutes
2. Converts database count → Prometheus metric
3. Prometheus then exposes: `auth_failures{reason="invalid_credentials"} 23`

**Why 5 minutes?**: Matches the alert evaluation window. If > 50 failures in 5 minutes, alert fires.

**PART 4: Background Collection Loop** (Lines 223-246)
```go
#### 🔍 Code Deep Dive: GCP Metrics Collection

**The Problem**: GKE (Google Kubernetes Engine) stores important metrics in GCP Cloud Monitoring, not in Kubernetes itself. We need to fetch them and convert to Prometheus format.

**PART 1: Counter Accumulator** (Lines 19-40)
```go
type counterAccumulator struct {
    mu   sync.Mutex                  // Thread safety (multiple goroutines)
    last map[string]float64          // Store previous value for each metric
}

func (c *counterAccumulator) delta(key string, current float64) float64 {
    c.mu.Lock()
    defer c.mu.Unlock()

    last, ok := c.last[key]           // Get previous value
    c.last[key] = current             // Store current value for next time
    
    if !ok {
        return 0                      // First time seeing this metric
    }
    if current < last {
        return current                 // Counter reset (pod restarted)
    }
    return current - last             // Normal case: delta
}
```
**Why this is needed**: GCP returns *cumulative* counters (total since container started). Prometheus needs *rate* (change per second). This calculates the difference between scrapes.

**Example**:
```
First scrape:  cpu_usage = 100 seconds → delta = 0 (first time)
Second scrape: cpu_usage = 115 seconds → delta = 15 seconds (used 15s in 30s window)
Third scrape:  cpu_usage = 133 seconds → delta = 18 seconds
```

**PART 2: GCP Client Setup** (Lines 159-179)
```go
func NewGCPExporter(projectID, clusterName string) (*GCPExporter, error) {
    ctx := context.Background()
    
    // Create GCP Monitoring API client
    client, err := monitoring.NewMetricClient(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to create monitoring client: %v", err)
    }

    return &GCPExporter{
        client:      client,
        projectID:   projectID,      // "dop-assignment-team1"
        clusterName: clusterName,    // "ai-model-tester-2"
    }, nil
}
```
**What this does**: Initializes Google Cloud Monitoring API client. Uses Application Default Credentials (service account attached to Kubernetes pod).

**PART 3: Collect Node CPU** (Lines 221-248)
```go
func (e *GCPExporter) collectNodeCPU(ctx context.Context, startTime, endTime time.Time) error {
    // Build query for GCP Monitoring API
    req := &monitoringpb.ListTimeSeriesRequest{
        Name:   "projects/" + e.projectID,
        
        // Filter to specific metric and cluster
        Filter: fmt.Sprintf(
            `metric.type="kubernetes.io/node/cpu/allocatable_utilization" AND resource.label.cluster_name="%s"`,
            e.clusterName,
        ),
        
        // Time range: last 5 minutes
        Interval: &monitoringpb.TimeInterval{
            EndTime:   timestamppb.New(endTime),
            StartTime: timestamppb.New(startTime),
        },
    }

    // Execute query (returns iterator)
    it := e.client.ListTimeSeries(ctx, req)
    
    // Loop through results (one per node)
    for {
        resp, err := it.Next()
        if err == iterator.Done {
            break  // No more results
        }
        if err != nil {
            return err
        }

        // Extract labels from response
        nodeName := resp.Resource.Labels["node_name"]        // e.g., "gke-cluster-node-1"
        clusterName := resp.Resource.Labels["cluster_name"]

        // Get most recent data point
        if len(resp.Points) > 0 {
            value := resp.Points[0].Value.GetDoubleValue()  // Returns 0.0-1.0
            
            // Update Prometheus metric (convert to percentage)
            gkeNodeCPUUtilization.WithLabelValues(nodeName, clusterName).Set(value * 100)
        }
    }

    return nil
}
```
**What this does**:
1. Queries GCP: "Give me CPU utilization for all nodes in cluster 'ai-model-tester-2'"
2. GCP returns time series data (one per node)
3. Extracts node name and latest value
4. Updates Prometheus gauge: `gke_node_cpu_utilization{node_name="gke-node-1"} 67.3`

**Data Flow**:
```
GCP Cloud Monitoring
    ↓ (API call every 30s)
GCP Exporter (fetches raw data)
    ↓ (converts to Prometheus format)
Prometheus Metric: gke_node_cpu_utilization{node_name="..."} 67.3
    ↓ (scraped by Prometheus)
Prometheus TSDB (stores historical data)
    ↓ (queried by Grafana)
Grafana Dashboard (shows graph)
```

**PART 4: Collect Container CPU** (Lines 287-317)
```go
func (e *GCPExporter) collectContainerCPU(ctx context.Context, startTime, endTime time.Time) error {
    req := &monitoringpb.ListTimeSeriesRequest{
        Name:   "projects/" + e.projectID,
        Filter: fmt.Sprintf(
            `metric.type="kubernetes.io/container/cpu/core_usage_time" AND resource.label.cluster_name="%s"`,
            e.clusterName,
        ),
        Interval: &monitoringpb.TimeInterval{
            EndTime:   timestamppb.New(endTime),
            StartTime: timestamppb.New(startTime),
        },
    }

    it := e.client.ListTimeSeries(ctx, req)
    for {
        resp, err := it.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            return err
        }

        // Extract container details
        containerName := resp.Resource.Labels["container_name"]
        podName := resp.Resource.Labels["pod_name"]
        namespace := resp.Resource.Labels["namespace_name"]

        if len(resp.Points) > 0 {
            value := resp.Points[0].Value.GetDoubleValue()  // Cumulative CPU seconds
            
            // Calculate delta (change since last scrape)
            key := containerName + "\x1f" + podName + "\x1f" + namespace  // Unique key
            delta := containerCPUAccumulator.delta(key, value)
            
            if delta > 0 {
                // Increment Prometheus counter by delta
                gkeContainerCPUUsage.WithLabelValues(containerName, podName, namespace).Add(delta)
            }
        }
    }

    return nil
}
```
**What this does**:
1. Queries GCP for container CPU (cumulative counter)
2. Uses `counterAccumulator` to calculate delta between scrapes
3. Only increments Prometheus counter if delta > 0
4. Result: `gke_container_cpu_usage_seconds{container_name="api-gateway",pod_name="...",namespace="dev"} 234.5`

**Why delta calculation?**:
- GCP returns: "Container has used 234.5 CPU seconds since start"
- Prometheus needs: "Container used 2.3 CPU seconds in last 30 seconds"
- Delta calculation converts cumulative → rate

**PART 5: Background Collection Loop** (Lines 512-532)
```go
func main() {
    projectID := getEnv("GCP_PROJECT_ID", "dop-assignment-team1")
    clusterName := getEnv("GKE_CLUSTER_NAME", "ai-model-tester-2")
    port := getEnv("PORT", "8888")

    exporter, err := NewGCPExporter(projectID, clusterName)
    if err != nil {
        log.Fatalf("Failed to create GCP exporter: %v", err)
    }
    defer exporter.Close()

    // Start background metric collection
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for {
            ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
            
            // Collect all metric types
            if err := exporter.collectMetrics(ctx); err != nil {
                log.Printf("Error collecting metrics: %v", err)
            }
            
            cancel()
            <-ticker.C  // Wait for next tick
        }
    }()

    // Expose /metrics endpoint for Prometheus
    http.Handle("/metrics", promhttp.Handler())
    
    // Start HTTP server
    log.Printf("Starting GCP monitoring exporter on port %s", port)
    server := &http.Server{Addr: ":" + port}
    log.Fatal(server.ListenAndServe())
}
```
**What this does**:
1. Every 30 seconds, fetch data from GCP Cloud Monitoring API
2. Convert to Prometheus metrics
3. Store in memory (Prometheus library handles this)
4. When Prometheus scrapes `/metrics`, return latest values
5. Timeout: 25 seconds (leaves 5s buffer before next tick)
**What this does**:
1. Creates a ticker that "ticks" every 30 seconds
2. Runs in background (`go func()`) without blocking main program
3. Each tick: queries database, updates Prometheus metrics
4. Uses timeout context (10s) to prevent hanging queries

**PART 5: Exposing Metrics** (Lines 260-292)
```go
func main() {
    // Try to connect to database
    collector, err := NewMetricsCollector()
    if err != nil {
        log.Printf("Warning: Could not connect to database: %v", err)
        log.Println("Continuing without database metrics...")
    } else {
        collector.StartCollecting()  // Start background collection
    }

    // Expose metrics at /metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    
    // Health check endpoint
    http.HandleFunc("/health", healthHandler)

    // Start HTTP server
    port := getEnv("PORT", "9090")
    server := &http.Server{
        Addr:         ":" + port,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    log.Fatal(server.ListenAndServe())
}
```
**What this does**:
1. Starts background metric collection (every 30s)
2. Starts HTTP server on port 9090
3. When Prometheus scrapes `http://metrics-exporter:9090/metrics`, it gets:
```
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{service="api-gateway",method="GET",status="200"} 1234

# HELP auth_failures Recent authentication failures in the last 5 minutes
# TYPE auth_failures gauge
auth_failures{reason="invalid_credentials"} 23

# HELP service_uptime_seconds Service uptime in seconds
# TYPE service_uptime_seconds gauge
service_uptime_seconds 3600
```

**Graceful Degradation**: If database is unreachable, exporter still runs (without DB metrics). This prevents monitoring from failing completely.hat this does**:
1. Takes Alertmanager payload (can have 1-100 alerts)
2. Limits to first 10 (Discord message size limit)
3. For each alert:
   - Extracts alertname, severity, summary, description
   - Chooses emoji and color (🔴 for critical, 🟡 for warning)
   - Builds structured Discord embed with fields
4. If > 10 alerts, adds "Showing 10 of 45 alerts..." message

**PART 5: Helper Functions**
```go
func severityToColor(severity string, isResolved bool) int {
    if isResolved {
        return 0x27AE60  // Green - problem fixed!
    }
    switch strings.ToLower(severity) {
    case "critical":
        return 0xD32F2F  // Red
    case "warning":
        return 0xFFA500  // Orange
    case "info":
        return 0x0099FF  // Blue
    default:
        return 0x808080  // Gray
    }
}

func getEmoji(severity string) string {
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
```
**What this does**: Maps severity levels to visual indicators. Makes alerts instantly recognizable at a glance.

**PART 6: Post to Discord** (Lines 238-275)
```go
func postDiscord(url, message string) error {
    // Parse Alertmanager JSON
    var alerts AlertmanagerPayload
    json.Unmarshal([]byte(message), &alerts)

    // Convert to Discord embeds
    embeds := buildEmbeds(alerts)
    
    // Build Discord message
    payload := DiscordMessage{
        Embeds:   embeds,
        Username: "Monitoring Alert",
    }
    
    // Convert to JSON bytes
    body, _ := json.Marshal(payload)

    // Create HTTP POST request
    req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    // Send request with 10 second timeout
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("post to discord: %w", err)
    }
    defer resp.Body.Close()

    // Check if Discord accepted it (2xx status code)
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("discord webhook returned %s", resp.Status)
    }

    return nil  // Success!
}
```
**What this does**:
1. Converts Alertmanager JSON → Discord embeds format
2. Marshals to JSON
3. Sends HTTP POST to Discord webhook URL
4. Returns error if Discord rejects (invalid webhook, rate limited, etc.)

**Visual Flow**:
```
Alertmanager: {"status":"firing","alerts":[{...}]}
       ↓
Discord Relay: Transforms to Discord format
       ↓
Discord API: Displays as rich embed with colors/emojis
       ↓
Discord Channel: 🔴 HighLatency [CRITICAL]
                 Category: Performance
                 Instance: api-gateway-abc123
Reads Discord webhook URLs from environment variables
2. Creates 3 different endpoints (critical/warning/main)
3. Each endpoint forwards to different Discord channels
4. Starts HTTP server listening on port 8080

**PART 3: Request Handler Factory** (Lines 75-113)
```go
func makeHandler(urlsEnv string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Only accept POST requests
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }
        
        // Parse comma-separated URLs: "url1,url2,url3" → ["url1", "url2", "url3"]
        urls := splitCSV(urlsEnv)
        if len(urls) == 0 {
            http.Error(w, "no discord webhook urls configured", http.StatusInternalServerError)
            return
        }

        // Decode JSON from Alertmanager
        var payload AlertmanagerPayload
        decoder := json.NewDecoder(r.Body)
        if err := decoder.Decode(&payload); err != nil {
            http.Error(w, "invalid json payload", http.StatusBadRequest)
            return
        }

        // Post to ALL Discord URLs (critical channel gets phone call too)
        for _, url := range urls {
            if err := postDiscord(url, alertJSON); err != nil {
                http.Error(w, err.Error(), http.StatusBadGateway)
                return
            }
        }

        w.WriteHeader(http.StatusOK)  // Success!
    }
}
```
**What this does**: Creates a handler function that:
1. Validates it's a POST request
2. Parses the Discord webhook URLs 
3. Decodes Alertmanager's JSON payload
4. Loops through ALL webhook URLs and posts to each
5. Returns success/error

**Why multiple URLs?**: For critical alerts, we send to Discord channel AND phone call service (PushCall) simultaneously.

# Resource Alert: High CPU usage
- alert: HighCPUUsage
  expr: pod_cpu_usage_percent > 80
  for: 10m
  labels:
    severity: warning

# Heartbeat: Regular system health updates (every 5 minutes)
- alert: SystemHeartbeat
  expr: <complex expression combining multiple metrics>
  labels:
    severity: info
    category: heartbeat
```

**Deployment Configuration**:
- **Retention**: 7 days (`--storage.tsdb.retention.time=7d`)
- **Resources**: 250m-1000m CPU, 512Mi-2Gi memory
- **Storage**: 2Gi emptyDir volume (ephemeral)

---

### 2. Alertmanager (`alertmanager/`)

**Purpose**: Handles alerts from Prometheus, deduplicates them, groups them, and routes them to the correct notification channels.

#### Files:
- **`alertmanager-config.yaml`**: Routing and receiver configuration
- **`deployment.yaml`**: Kubernetes deployment
- **`service.yaml`**: Network service

#### Routing Logic (`alertmanager-config.yaml`):

```yaml
route:
  group_by: ['alertname', 'cluster', 'service']
  group_wait: 30s        # Wait 30s to collect related alerts
  group_interval: 5m     # Send grouped alerts every 5 minutes
  repeat_interval: 12h   # Resend unresolved alerts every 12 hours
  
  routes:
    # Special case: Heartbeat alerts every 5 minutes
    - match:
        alertname: SystemHeartbeat
      receiver: 'default-receiver'
      repeat_interval: 5m
      continue: false    # Stop routing after this match
    
    # Critical alerts → Developer + QA channels + Phone call
    - match:
        severity: critical
      receiver: 'discord-critical'
      continue: false
    
    # Warning alerts → QA channel only
    - match:
        severity: warning
      receiver: 'discord-warning'
      continue: false
```

**Receivers** (Where alerts go):
- **`default-receiver`**: Main Discord channel (heartbeats, info)
- **`discord-critical`**: Multiple channels + phone call webhook
- **`discord-warning`**: QA Discord channel only

All receivers point to the Discord Relay service (explained below).

---

### 3. Discord Relay (`discord-relay/`)

**Purpose**: Translates Alertmanager's webhook format into beautiful Discord embed messages and posts to multiple Discord channels/webhooks.

#### Files:
- **`main.go`**: Go application code (304 lines)
- **`deployment.yaml`**: Kubernetes deployment
- **`service.yaml`**: Internal service (port 8080)
- **`Dockerfile`**: Container image build
- **`go.mod`**: Go dependencies

#### How It Works:

**HTTP Endpoints**:
- `/healthz`: Health check endpoint
- `/webhook/critical`: Receives critical severity alerts
- `/webhook/warning`: Receives warning severity alerts
- `/webhook/main`: Receives info/heartbeat alerts

**Key Code Functions**:

1. **`makeHandler(urlsEnv string)`** (Line 75-113):
   ```go
   // Creates an HTTP handler for each severity level
   // Reads Discord webhook URLs from environment variable (comma-separated)
   // Decodes Alertmanager JSON payload
   // Posts to all configured Discord webhooks
   ```

2. **`buildEmbeds(payload AlertmanagerPayload)`** (Line 115-203):
   ```go
   // Converts Alertmanager alerts into Discord embed format
   // Limits to first 10 alerts (prevents message size issues)
   // Extracts: alertname, severity, summary, description, instance
   // Creates color-coded embeds:
   //   - Critical: 🔴 Red (0xD32F2F)
   //   - Warning: 🟡 Orange (0xFFA500)
   //   - Info: ℹ️ Blue (0x0099FF)
   //   - Resolved: Green (0x27AE60)
   ```

3. **`postDiscord(url, message string)`** (Line 238-275):
   ```go
   // Marshals Discord message to JSON
   // POSTs to Discord webhook URL
   // Timeout: 10 seconds per request
   // Returns error if Discord returns non-2xx status
   ```

**Environment Variables**:
- `DISCORD_CRITICAL_URLS`: Comma-separated webhook URLs for critical alerts
- `DISCORD_WARNING_URLS`: Comma-separated webhook URLs for warnings
- `DISCORD_MAIN_URLS`: Main channel for heartbeats and info

Example Discord URLs:
```
DISCORD_CRITICAL_URLS=
  https://discord.com/api/webhooks/XXX/YYY,
  https://pushcall.me/api/call?api_key=ZZZ&from=XXX&to=YYY
```

---

### 4. Metrics Exporter (`metrics-exporter/`)

**Purpose**: Custom exporter that connects to your application's PostgreSQL database and exposes application-specific metrics in Prometheus format.

#### Files:
- **`main.go`**: Go application (292 lines)
- **`deployment.yaml`**: Kubernetes deployment
- **`Dockerfile`**: Container image

#### What It Exports:

**HTTP Request Metrics**:
```go
http_requests_total{service="api-gateway",method="GET",status="200"} 1234
http_request_duration_seconds{service="api-gateway",method="GET"}
```

**Authentication Metrics**:
```go
auth_failures{reason="invalid_credentials"} 45
// Counts from auth_events table in last 5 minutes
```

**User Metrics**:
```go
active_users_total 523
// Users who logged in within last 24 hours

user_registrations_total 10234
// Total user count
```

**Security Events**:
```go
security_events{event_type="auth_failure",severity="critical"} 12
```

**Database Queries** (Line 142-149):
```sql
-- Count authentication failures in last 5 minutes
SELECT COUNT(*) FROM auth_events 
WHERE event_type = 'auth_failure' 
AND created_at > NOW() - INTERVAL '5 minutes'
```

**Collection Interval**: Every 30 seconds

---

### 5. GCP Exporter (`gcp-exporter/`)

**Purpose**: Fetches metrics from GCP Cloud Monitoring API (metrics that exist outside Kubernetes) and exposes them in Prometheus format. Essential for GKE-managed metrics.

#### Files:
- **`main.go`**: Go application (542 lines)
- *🎓 How to Explain This to Your Teacher

### Quick Elevator Pitch (30 seconds)
"We built a complete monitoring system for our Kubernetes application. Prometheus collects metrics from all our services every 15 seconds. When something goes wrong - like high CPU, slow response times, or security issues - it triggers alerts that get sent to Discord channels with different priorities. Critical alerts go to developers AND trigger a phone call. We also pull metrics from Google Cloud to monitor our GKE cluster health."

### Detailed Explanation (5 minutes)

**1. The Core Architecture**
```
Our apps expose /metrics endpoints → Prometheus scrapes them every 15s → 
Stores in time-series database → Evaluates alerting rules → 
Alertmanager routes by severity → Discord Relay formats messages → 
Discord channels (team gets notified)
```

**2. Why We Built Custom Exporters**

**Teacher might ask**: "Why not use existing tools?"

**Your answer**: 
- **Metrics Exporter**: Our application stores auth failures and user activity in PostgreSQL. Standard exporters don't understand our database schema. Our custom exporter queries specific tables and exposes them as Prometheus metrics.
- **GCP Exporter**: GKE stores important metrics in Google Cloud Monitoring (CPU, memory per node). Prometheus can't access GCP APIs directly. Our exporter fetches GCP metrics every 30s and converts them to Prometheus format.
- **Discord Relay**: Alertmanager only supports basic webhooks. Discord requires rich embeds with colors, emojis, and formatted fields. Our relay translates alert JSON into beautiful Discord messages.

**3. Key Design Decisions**

**Q**: "Why use recording rules?"
**A**: We pre-compute expensive queries (like average latency) every 15 seconds. This makes dashboards load instantly instead of recalculating on every refresh. It's like caching.

**Q**: "Why send heartbeat alerts every 5 minutes?"
**A**: Proves the monitoring system itself is working. If heartbeats stop, we know monitoring is down, not that everything is perfect. It's a "dead man's switch."

**Q**: "Why different Discord channels for severity levels?"
**A**: Critical alerts (system down, high error rate) need immediate attention from developers. Warnings (high CPU) can be handled by QA during business hours. This prevents alert fatigue and ensures the right people see the right alerts.

**Q**: "How do you handle counter resets?"
**A**: GCP returns cumulative counters (total since container start). When a pod restarts, the counter resets to 0. Our `counterAccumulator` detects this (current < last) and handles it gracefully, preventing negative values.

**4. Technical Highlights to Mention**

**Service Discovery**:
"We use Kubernetes service discovery with annotations. Any service tagged with `prometheus.io/scrape: true` is automatically monitored. No manual configuration needed when we deploy new services."

**RBAC Security**:
"Prometheus needs read access to the Kubernetes API to discover services. We created a ServiceAccount with ClusterRole that only allows GET/LIST/WATCH - principle of least privilege."

**Graceful Degradation**:
"If the database is down, the metrics exporter continues running (just without DB metrics). This prevents cascading failures where monitoring breaks when apps break."

**Data Retention**:
"We keep 7 days of metrics in Prometheus (limited by memory/disk). For longer retention, we could export to BigQuery or use Thanos."

### Common Teacher Questions & Answers

**Q**: "What happens if Prometheus goes down?"
**A**: We lose monitoring data during the outage, but historical data is preserved. When it restarts, it resumes scraping. In production, we'd run multiple Prometheus replicas with shared storage (Thanos) for high availability.

**Q**: "How do you prevent false positives?"
**A**: We use the `for` duration in alerts. For example, high CPU must persist for 10 minutes before firing. This filters out temporary spikes during deployments.

**Q**: "What's the performance impact of scraping every 15 seconds?"
**A**: Minimal. Each scrape is a single HTTP GET request to `/metrics`. The overhead is ~1-5ms per scrape. With 20 targets, that's ~100ms total every 15 seconds.

**Q**: "How do you secure Discord webhooks?"
**A**: Webhook URLs are stored in Kubernetes ConfigMaps (ideally Secrets in production). They're not in code. Discord webhook URLs act as passwords - anyone with the URL can post to that channel, so we keep them secret.

**Q**: "What if you get rate limited?"
**A**: Discord allows 30 messages per minute per webhook. We group alerts (wait 30s, send multiple in one message) and use multiple webhook URLs for critical alerts. The phone call service has separate limits.

### Demo Flow for Teacher

1. **Show Prometheus UI** (`localhost:9090`)
   - Show targets: all green = scraping successfully
   - Run query: `rate(http_requests_total[5m])` → Shows request rate
   - Show alerts: which are firing/pending

2. **Show Grafana Dashboard** (`localhost:3000`)
   - Point out different panels: CPU, memory, request rate, latency
   - Explain data source: queries Prometheus
   - Show time range selector (last 1 hour, last 24 hours)

3. **Show Alertmanager** (`localhost:9093`)
   - Show active alerts
   - Show silences (temporarily mute alerts)
   - Explain routing tree: severity → receiver

4. **Show Discord Channel**
   - Point out color coding (red=critical, orange=warning)
   - Show heartbeat messages (proves system is alive)
   - Show alert resolution messages (green)

5. **Trigger a Test Alert** (Optional)
   ```bash
   # Artificially increase a metric to trigger alert
   kubectl scale deployment api-gateway --replicas=0
   # Wait 2 minutes → ServiceDown alert fires → Discord message
   kubectl scale deployment api-gateway --replicas=1
   # Alert resolves → Discord shows green message
   ```

---

## 📚 Learning Resources & References

### Understanding the Stack
- **Prometheus Docs**: https://prometheus.io/docs/introduction/overview/
- **PromQL Tutorial**: https://prometheus.io/docs/prometheus/latest/querying/basics/
- **Grafana Getting Started**: https://grafana.com/docs/grafana/latest/getting-started/
- **Alertmanager Configuration**: https://prometheus.io/docs/alerting/latest/configuration/
- **Discord Webhooks**: https://discord.com/developers/docs/resources/webhook

### Deep Dives
- **Kubernetes Service Discovery**: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config
- **Recording Rules Best Practices**: https://prometheus.io/docs/practices/rules/
- **GCP Monitoring API**: https://cloud.google.com/monitoring/api/v3
- **Go Prometheus Client**: https://github.com/prometheus/client_golang
- **PostgreSQL Performance**: https://www.postgresql.org/docs/current/performance-tips.html

### Alternative Approaches (What We Could Have Done)
- **Datadog/New Relic**: Paid SaaS monitoring (easier setup, more expensive)
- **ELK Stack**: Better for logs than metrics
- **Thanos**: For long-term Prometheus storage
- **PagerDuty**: Professional on-call management (instead of Discord)
- **Prometheus Operator**: Kubernetes-native Prometheus deployment

---

## 🔬 Hands-On Exercises to Deepen Understanding

### Exercise 1: Write a Custom PromQL Query
Query Prometheus for "pods using more than 50% memory":
```promql
(container_memory_working_set_bytes / container_spec_memory_limit_bytes * 100) > 50
```

### Exercise 2: Add a New Alert
Create an alert for "No heartbeat in 10 minutes":
```yaml
- alert: MonitoringDown
  expr: absent_over_time(ALERTS{alertname="SystemHeartbeat"}[10m])
  labels:
    severity: critical
  annotations:
    summary: "Monitoring system is not sending heartbeats"
```

### Exercise 3: Trace a Metric End-to-End
Pick one metric: `http_requests_total`
1. Find where it's incremented in application code
2. View raw metric at `/metrics` endpoint
3. Query in Prometheus UI
4. See it visualized in Grafana
5. Check if any alerts use it

### Exercise 4: Simulate an Alert
```bash
# Scale deployment to 0 replicas → triggers ServiceDown alert
kubectl scale deployment api-gateway -n dev --replicas=0

# Wait 2-3 minutes, check:
# 1. Prometheus UI → Alerts tab (should show firing)
# 2. Alertmanager UI → Should show alert
# 3. Discord channel → Should receive message

# Resolve by scaling back up
kubectl scale deployment api-gateway -n dev --replicas=1
```

### Exercise 5: Debug a Scraping Issue
```bash
# Check if Prometheus is scraping a target
kubectl port-forward -n dev svc/prometheus 9090:9090

# Open http://localhost:9090/targets
# Look for red targets

# Debug steps:
# 1. Check if pod is running: kubectl get pods -n dev
# 2. Check pod annotations: kubectl describe pod <name> -n dev
# 3. Test metrics endpoint: kubectl exec -n dev <pod> -- wget -O- localhost:9090/metrics
# 4. Check Prometheus configmap: kubectl get cm prometheus-config -n dev -o yaml
```

---

## Summary

This monitoring stack provides:
- ✅ **Comprehensive observability** of your Kubernetes cluster and applications
- ✅ **Real-time alerting** to Discord channels with severity-based routing
- ✅ **Custom metrics** from your application database
- ✅ **GCP integration** for GKE-specific metrics
- ✅ **Beautiful visualizations** in Grafana
- ✅ **Single-command deployment** via Kustomize

**Data Path**: Applications → Exporters → Prometheus → Alertmanager → Discord Relay → Discord

**Key Innovation**: The Discord Relay service converts technical Alertmanager payloads into beautifully formatted Discord embeds with color coding, emojis, and structured information, making alerts actionable and easy to understand at a glance.

**Learning Takeaway**: This project demonstrates understanding of:
- Time-series databases (Prometheus)
- Service mesh observability (Kubernetes annotations)
- Distributed systems monitoring (multiple exporters)
- Alert routing and notification systems (Alertmanager + Discord)
- Cloud platform integration (GCP API)
- Go programming (custom exporters)
- SQL database queries (metrics collection)
- Data visualization (Grafana dashboards)
gke_pod_network_received_bytes_total{pod_name="...",namespace="dev"} 1048576000
gke_pod_network_sent_bytes_total{pod_name="...",namespace="dev"} 524288000
```

**Key Code Functions**:

1. **`collectNodeCPU()`** (Line 221-248):
   ```go
   // Query: kubernetes.io/node/cpu/allocatable_utilization
   // Returns CPU utilization as percentage (0-1, converted to 0-100)
   // Uses GCP Monitoring API ListTimeSeries
   ```

2. **`collectContainerCPU()`** (Line 287-317):
   ```go
   // Query: kubernetes.io/container/cpu/core_usage_time
   // Cumulative counter - calculates delta between scrapes
   // Uses counterAccumulator to handle counter resets
   ```

3. **`collectPodNetwork()`** (Line 355-413):
   ```go
   // Queries both received and sent bytes counters
   // Calculates deltas to expose as Prometheus counters
   // Handles counter resets gracefully
   ```

**Authentication**: Uses GCP Application Default Credentials (service account with Monitoring Viewer role)

**Collection Interval**: Every 30 seconds

---

### 6. Grafana (`grafana/`)

**Purpose**: Visualization platform for creating dashboards to display metrics from Prometheus and GCP.

#### Files:
- **`deployment.yaml`**: Main deployment + ConfigMaps + PVC
- **`dashboards/k8s-monitoring.json`**: Pre-configured dashboard

#### Configuration (`deployment.yaml`):

**Data Sources**:
```yaml
datasources:
  - name: Prometheus
    url: http://prometheus:9090
    isDefault: true
    
  - name: Google Cloud Monitoring
    type: stackdriver
    authenticationType: gce              # Uses GKE service account
    defaultProject: dop-assignment-team1
    
  - name: Google Cloud Logging  
    type: stackdriver
    logType: cloud_logging
```

**Dashboard Provider**:
```yaml
providers:
  - name: 'default'
    folder: ''
    type: file
    path: /var/lib/grafana/dashboards     # Loads k8s-monitoring.json
    updateIntervalSeconds: 10
    allowUiUpdates: true                  # Users can edit dashboards
```

**Storage**: 10Gi PersistentVolumeClaim for dashboard persistence

**Default Credentials**: admin/admin (should be changed in production!)

---

### 7. Node Exporter (`exporters/node-exporter.yaml`)

**Purpose**: Runs on every Kubernetes node (DaemonSet) to collect host-level metrics like CPU, memory, disk, network interfaces.

**Deployment Type**: DaemonSet (1 pod per node)

**Host Access**:
```yaml
hostNetwork: true    # Access to host network stack
hostPID: true        # Access to host process tree
hostIPC: true        # Access to host IPC namespace
```

**Mounted Paths**:
- `/host/proc` → Host's `/proc` filesystem (process stats)
- `/host/sys` → Host's `/sys` filesystem (system stats)
- `/rootfs` → Host's root filesystem (disk stats)

**Prometheus Annotations**:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9100"
```

**Metrics Exposed**: CPU, memory, disk I/O, network I/O, filesystem usage, load average, etc.

---

### 8. Kube-State-Metrics (`exporters/kube-state-metrics.yaml`)

**Purpose**: Exposes Kubernetes object state metrics (deployments, pods, nodes, services status) that Kubernetes itself doesn't expose.

**Examples of Metrics**:
```
kube_pod_status_phase{pod="api-gateway-xxx",phase="Running"} 1
kube_deployment_replicas{deployment="prometheus"} 1
kube_deployment_replicas_available{deployment="prometheus"} 1
kube_node_status_condition{node="node-1",condition="Ready",status="true"} 1
kube_pod_container_status_restarts_total{pod="api-gateway-xxx"} 3
```

**RBAC Permissions**: Requires ClusterRole to list/watch:
- Pods, Nodes, Services, Deployments
- StatefulSets, DaemonSets, Jobs, CronJobs
- ConfigMaps, Secrets, PersistentVolumes
- HorizontalPodAutoscalers

**Prometheus Annotations**:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

---

## Configuration Files

### `kustomization.yaml` (Root)

**Purpose**: Kustomize manifest that ties all monitoring components together. Single command deployment.

**Resources Included**:
```yaml
resources:
  - prometheus/serviceaccounts.yaml
  - prometheus/configmap.yaml
  - prometheus/pvc.yaml
  - prometheus/deployment.yaml
  - prometheus/services.yaml
  - grafana/deployment.yaml
  - alertmanager/deployment.yaml
  - alertmanager/service.yaml
  - metrics-exporter/deployment.yaml
  - exporters/node-exporter.yaml
  - exporters/kube-state-metrics.yaml
  - gcp-exporter/deployment.yaml
  - gcp-exporter/service.yaml
  - discord-relay/deployment.yaml
  - discord-relay/service.yaml
```

**ConfigMap Generators**:
```yaml
configMapGenerator:
  # Grafana dashboard loaded from JSON file
  - name: grafana-dashboards
    files:
      - grafana/dashboards/k8s-monitoring.json
  
  # Alertmanager config loaded from YAML file
  - name: alertmanager-config
    files:
      - alertmanager/alertmanager-config.yaml
  
  # Discord webhook URLs (inline literals)
  - name: discord-relay-config
    literals:
      - DISCORD_CRITICAL_URLS=https://discord.com/...
      - DISCORD_WARNING_URLS=https://discord.com/...
      - DISCORD_MAIN_URLS=https://discord.com/...
```

**Deployment Command**:
```bash
kubectl apply -k monitoring/
```

---

## Data Flow

### 1. Metrics Collection Flow

```
Application Services
    │
    │ (Annotated with prometheus.io/scrape: "true")
    │ Expose /metrics endpoint
    │
    ▼
┌─────────────────┐
│  PROMETHEUS     │◄───────── Scrapes every 15s
│                 │
│ - Stores data   │◄───────── Node Exporter (host metrics)
│ - Evaluates     │◄───────── Kube-State-Metrics (K8s objects)
│   rules         │◄───────── Metrics Exporter (database)
│ - Executes      │◄───────── GCP Exporter (GCP metrics)
│   queries       │
└─────────────────┘
    │
    │ (Recording rules evaluated every 15s)
    │ Pre-aggregate metrics
    │
    ├─────────────► api_requests_per_second
    ├─────────────► api_request_latency_avg_ms
    ├─────────────► pod_cpu_usage_percent
    └─────────────► node_memory_usage_percent
```

### 2. Alerting Flow

```
┌─────────────────┐
│  PROMETHEUS     │
│                 │
│ Evaluate        │
│ alerting_rules  │
│ every 15s       │
└─────────────────┘
    │
    │ Alert fires if condition met for duration
    │
    ▼
┌─────────────────┐
│ ALERTMANAGER    │
│                 │
│ - Groups alerts │
│ - Deduplicates  │
│ - Routes by     │
│   severity      │
└─────────────────┘
    │
    ├─── severity: critical ──► /webhook/critical ─────┐
    │                                                   │
    ├─── severity: warning ───► /webhook/warning ──────┤
    │                                                   │
    └─── default ─────────────► /webhook/main ─────────┤
                                                       │
                                                       ▼
                                            ┌─────────────────┐
                                            │ DISCORD RELAY   │
                                            │                 │
                                            │ - Formats       │
                                            │ - Beautifies    │
                                            │ - Posts to      │
                                            │   Discord       │
                                            └─────────────────┘
                                                       │
                                                       │
    ┌──────────────────────────────────────────────────┼──────────┐
    │                                                   │          │
    ▼                                                   ▼          ▼
Discord                                            Discord     Phone Call
Critical Channel                                   Warning     (PushCall)
(Developers)                                       Channel
                                                   (QA Team)
```

### 3. Visualization Flow

```
     ┌─────────────────┐
     │    GRAFANA      │
     │                 │
     │  User opens     │
     │  dashboard      │
     └─────────────────┘
              │
              │ Execute PromQL queries
              │
              ▼
     ┌─────────────────┐
     │  PROMETHEUS     │
     │                 │
     │  Returns        │
     │  time-series    │
     │  data points    │
     └─────────────────┘
              │
              │
              ▼
     ┌─────────────────┐
     │    GRAFANA      │
     │                 │
     │  Renders        │
     │  graphs,        │
     │  charts,        │
     │  panels         │
     └─────────────────┘
```

---

## Alert Routing

### Severity Levels and Destinations

| Severity | Condition | Route | Destination | Repeat Interval |
|----------|-----------|-------|-------------|-----------------|
| **critical** | System failures, high error rates, security breaches | `discord-critical` | 🔴 Developer Discord + QA Discord + Phone call | 12 hours |
| **warning** | High resource usage, moderate latency | `discord-warning` | 🟡 QA Discord only | 12 hours |
| **info** | Heartbeat, system is healthy | `default-receiver` | ℹ️ Main Discord (all) | 5 minutes |

### Alert Examples

**Critical Alert Example**:
```yaml
Alert: HighErrorRate
Severity: critical
Labels:
  service: api-gateway
  category: reliability
Annotations:
  summary: "High error rate detected"
  description: "Error rate is 8.5% for api-gateway"
```
→ Sent to: Developers + QA + Phone call

**Warning Alert Example**:
```yaml
Alert: HighCPUUsage
Severity: warning
Labels:
  pod: prometheus-abc123
  namespace: dev
  category: resources
Annotations:
  summary: "High CPU usage on pod"
  description: "Pod prometheus-abc123 in dev is using 82% CPU"
```
→ Sent to: QA team only

**Heartbeat Example**:
```yaml
Alert: SystemHeartbeat
Severity: info
Labels:
  metric: api_rps
  unit: rps
Annotations:
  summary: "api_rps: 45.2 rps"
  description: "System is healthy - api_rps = 45.2 rps"
```
→ Sent to: Main channel (all) every 5 minutes

---

## Deployment

### Prerequisites

1. **Kubernetes Cluster**: GKE cluster running
2. **kubectl**: Configured to access the cluster
3. **Namespace**: `dev` namespace must exist
4. **Secrets**: PostgreSQL secrets for metrics-exporter
5. **GCP Service Account**: For GCP Exporter with Monitoring Viewer role
6. **Discord Webhooks**: Create webhook URLs in Discord channels

### Step-by-Step Deployment

**1. Create Namespace** (if not exists):
```bash
kubectl create namespace dev
```

**2. Create PostgreSQL Secret** (if using metrics-exporter):
```bash
kubectl create secret generic postgres-secret -n dev \
  --from-literal=database=app_db \
  --from-literal=username=app_user \
  --from-literal=password=your_secure_password
```

**3. Update Discord Webhook URLs**:
Edit [kustomization.yaml](kustomization.yaml) and replace webhook URLs:
```yaml
configMapGenerator:
  - name: discord-relay-config
    literals:
      - DISCORD_CRITICAL_URLS=your_critical_webhook_url
      - DISCORD_WARNING_URLS=your_warning_webhook_url
      - DISCORD_MAIN_URLS=your_main_webhook_url
```

**4. Deploy Entire Stack**:
```bash
kubectl apply -k monitoring/
```

**5. Verify Deployments**:
```bash
kubectl get pods -n dev
```

Expected output:
```
NAME                                READY   STATUS    RESTARTS   AGE
prometheus-xxxx                     1/1     Running   0          2m
grafana-xxxx                        1/1     Running   0          2m
alertmanager-xxxx                   1/1     Running   0          2m
discord-relay-xxxx                  1/1     Running   0          2m
metrics-exporter-xxxx               1/1     Running   0          2m
gcp-exporter-xxxx                   1/1     Running   0          2m
node-exporter-xxxx                  1/1     Running   0          2m
kube-state-metrics-xxxx             1/1     Running   0          2m
```

**6. Port Forward for Local Access**:

Prometheus:
```bash
kubectl port-forward -n dev svc/prometheus 9090:9090
```
Access: http://localhost:9090

Grafana:
```bash
kubectl port-forward -n dev svc/grafana 3000:3000
```
Access: http://localhost:3000 (admin/admin)

Alertmanager:
```bash
kubectl port-forward -n dev svc/alertmanager 9093:9093
```
Access: http://localhost:9093

---

## Troubleshooting

### Common Issues

**1. Prometheus Not Scraping Targets**

Check targets status:
```bash
# Port forward Prometheus
kubectl port-forward -n dev svc/prometheus 9090:9090

# Open browser: http://localhost:9090/targets
```

Look for:
- ❌ Red targets = scraping failed
- ✅ Green targets = scraping successful

Common fixes:
- Verify pod annotations: `prometheus.io/scrape: "true"`
- Check service port matches annotation `prometheus.io/port`
- Verify pod is running and healthy

**2. Alerts Not Firing**

Check Prometheus alerts:
```bash
# Browser: http://localhost:9090/alerts
```

Verify:
- Alert rule syntax is correct
- Alert condition is actually met (run query)
- Alert `for` duration has elapsed

Check Alertmanager:
```bash
# Browser: http://localhost:9093
```

**3. Discord Messages Not Sending**

Check Discord Relay logs:
```bash
kubectl logs -n dev deployment/discord-relay
```

Common issues:
- Invalid webhook URLs (test manually with curl)
- Rate limiting by Discord (429 errors)
- Network egress blocked

Test webhook manually:
```bash
curl -X POST "YOUR_WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d '{"content": "Test message"}'
```

**4. GCP Exporter Not Collecting Metrics**

Check logs:
```bash
kubectl logs -n dev deployment/gcp-exporter
```

Common issues:
- GCP service account missing "Monitoring Viewer" role
- Wrong project ID or cluster name in environment variables
- Metrics don't exist in GCP (verify in GCP Console)

**5. Metrics Exporter Database Connection Failed**

Check logs:
```bash
kubectl logs -n dev deployment/metrics-exporter
```

Common issues:
- PostgreSQL secret not created or wrong credentials
- Database service name incorrect (should be `auth-postgres-service`)
- Database tables don't exist (auth_events, users, security_events)

**6. Grafana Dashboard Empty/No Data**

Check:
1. Prometheus data source connection (Grafana → Configuration → Data Sources)
2. Run queries manually in Prometheus first
3. Verify time range in dashboard (top right)
4. Check for metric name changes

---

## Service Ports Reference

| Service | Port | Purpose |
|---------|------|---------|
| Prometheus | 9090 | Query interface, metrics scraping |
| Grafana | 3000 | Web UI for dashboards |
| Alertmanager | 9093 | Alert management UI |
| Discord Relay | 8080 | Webhook receiver for alerts |
| Metrics Exporter | 9090 | Custom application metrics |
| GCP Exporter | 8888 | GCP metrics in Prometheus format |
| Node Exporter | 9100 | Host-level metrics |
| Kube-State-Metrics | 8080 | Kubernetes object metrics |

---

## Key Metrics Reference

### HTTP Metrics
```promql
# Request rate
rate(http_requests_total[5m])

# Latency (average)
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])
```

### Resource Metrics
```promql
# CPU usage per pod
rate(container_cpu_usage_seconds_total[5m]) * 100

# Memory usage per pod
container_memory_working_set_bytes / container_spec_memory_limit_bytes * 100

# Node CPU average
avg(gke_node_cpu_utilization)

# Node memory average
avg(gke_node_memory_utilization)
```

### Kubernetes Metrics
```promql
# Running pods count
count(kube_pod_status_phase{phase="Running"})

# Pod restart rate
rate(kube_pod_container_status_restarts_total[15m])

# Ready nodes
count(kube_node_status_condition{condition="Ready",status="true"})
```

---

## Security Considerations

1. **Grafana Password**: Change default admin/admin password
2. **Discord Webhooks**: Keep URLs secret (they allow posting to channels)
3. **Database Credentials**: Use Kubernetes secrets, not environment variables
4. **GCP Service Account**: Principle of least privilege (Monitoring Viewer only)
5. **RBAC**: Prometheus service account has cluster-wide read access
6. **Network Policies**: Consider restricting egress to only Discord webhooks

---

## Maintenance

### Regular Tasks

**Weekly**:
- Review dashboard for anomalies
- Check alert fatigue (too many false positives?)
- Verify all exporters are healthy

**Monthly**:
- Review and update alerting thresholds
- Clean up old dashboards
- Update container images to latest versions

**Quarterly**:
- Audit RBAC permissions
- Review and optimize high-cardinality metrics
- Test disaster recovery (delete Prometheus, verify recovery)

---

## Additional Resources

- **Prometheus Docs**: https://prometheus.io/docs/
- **Grafana Docs**: https://grafana.com/docs/
- **Alertmanager Docs**: https://prometheus.io/docs/alerting/latest/alertmanager/
- **PromQL Guide**: https://prometheus.io/docs/prometheus/latest/querying/basics/
- **GCP Monitoring API**: https://cloud.google.com/monitoring/api/v3

---

## Summary

This monitoring stack provides:
- ✅ **Comprehensive observability** of your Kubernetes cluster and applications
- ✅ **Real-time alerting** to Discord channels with severity-based routing
- ✅ **Custom metrics** from your application database
- ✅ **GCP integration** for GKE-specific metrics
- ✅ **Beautiful visualizations** in Grafana
- ✅ **Single-command deployment** via Kustomize

**Data Path**: Applications → Exporters → Prometheus → Alertmanager → Discord Relay → Discord

**Key Innovation**: The Discord Relay service converts technical Alertmanager payloads into beautifully formatted Discord embeds with color coding, emojis, and structured information, making alerts actionable and easy to understand at a glance.
