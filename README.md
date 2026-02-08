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

#### Key Configuration (`configmap.yaml`):

```yaml
# Global scrape settings
scrape_interval: 15s      # Collect metrics every 15 seconds
evaluation_interval: 15s  # Evaluate alerting rules every 15 seconds
```

**Scrape Configurations** (What Prometheus monitors):

1. **`kubernetes-apiservers`**: Monitors the Kubernetes API server itself
   - Authentication via service account tokens
   - Uses TLS for secure communication
   
2. **`kubernetes-nodes`**: Node-level metrics
   - CPU, memory, disk usage per node
   - Network traffic

3. **`kubernetes-service-endpoints`**: Services with annotation `prometheus.io/scrape: "true"`
   - Discovers services automatically
   - Extracts metrics from custom ports/paths

4. **`kubernetes-pods`**: Pod-level metrics with annotation `prometheus.io/scrape: "true"`
   - Container-specific metrics
   - Application custom metrics

5. **`gcp-monitoring`**: GCP Cloud Monitoring data via custom exporter
   - GKE-specific metrics
   - 30-second scrape interval

**Recording Rules** (Pre-computed metrics for performance):

```yaml
# Calculate requests per second across all services
- record: api_requests_per_second
  expr: sum(rate(http_requests_total[1m])) by (service, method, status)

# Calculate average API latency in milliseconds
- record: api_request_latency_avg_ms
  expr: sum(rate(http_request_duration_seconds_sum[1m])) / 
        sum(rate(http_request_duration_seconds_count[1m])) * 1000

# Calculate error rate (5xx errors / total requests)
- record: api_error_rate
  expr: sum(rate(http_requests_total{status=~"5.."}[1m])) / 
        sum(rate(http_requests_total[1m]))
```

**Alerting Rules** (When to trigger alerts):

```yaml
# Security Alert: Too many auth failures
- alert: HighAuthFailureRate
  expr: sum(auth_failures) > 50
  for: 2m                        # Must persist for 2 minutes
  labels:
    severity: critical
    category: security

# Performance Alert: High latency
- alert: HighLatency
  expr: api_request_latency_avg_ms > 1000  # Over 1 second
  for: 5m
  labels:
    severity: warning

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
- **`deployment.yaml`**: Kubernetes deployment
- **`service.yaml`**: Internal service (port 8888)
- **`docker-compose.yml`**: Local testing setup
- **`Dockerfile`**: Container image
- **`go.mod`**: Go dependencies (GCP SDK)

#### GCP Metrics Collected:

**Node-Level Metrics**:
```go
gke_node_cpu_utilization{node_name="gke-node-1",cluster_name="..."} 45.2
gke_node_memory_utilization{node_name="gke-node-1"} 67.8
gke_node_disk_utilization{node_name="gke-node-1"} 34.1
```

**Container Metrics**:
```go
gke_container_cpu_usage_seconds{container_name="api-gateway",pod_name="...",namespace="dev"} 234.5
gke_container_memory_usage_bytes{container_name="api-gateway"} 536870912
```

**Pod Network Metrics**:
```go
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
