# Monitoring Folder Documentation

This folder contains the complete monitoring, alerting, and observability stack for the DevOps system.

---

## 📁 Folder Structure

```
monitoring/
├── kustomization.yaml          # Master config - deploys all monitoring components
├── prometheus/                 # Time-series database for metrics
├── grafana/                    # Dashboard visualization
├── alertmanager/               # Alert routing engine
├── discord-relay/              # Discord notification bridge
├── metrics-exporter/           # Custom app metrics collector
├── gcp-exporter/               # GCP Cloud Monitoring metrics collector
└── exporters/                  # System-level metrics exporters
```

---

## 🎯 Root Level Files

### **kustomization.yaml**
- **Purpose**: Master configuration file that orchestrates all monitoring components
- **What it does**:
  - Lists all YAML manifests to deploy in Kubernetes
  - Generates ConfigMaps for Grafana dashboards and AlertManager config
  - Injects environment variables for Discord relay
  - Prevents hash suffixes on config names (keeps names stable)
- **Key sections**:
  - `resources`: Lists all manifests to apply
  - `configMapGenerator`: Creates ConfigMaps from files and literals
  - `DISCORD_*_URLS`: Environment variables for notification channels

**Example resources deployed**:
- Prometheus (serviceaccounts, configmap, pvc, deployment, services)
- Grafana (deployment, dashboards)
- AlertManager (deployment, service)
- All exporters (metrics, gcp, node, kube-state)
- Discord Relay (deployment, service)

---

## 📊 Prometheus (`/prometheus`)

**Time-series database that stores all metrics over time**

### **Files**

#### **deployment.yaml**
- **Kind**: Kubernetes Deployment
- **Container**: `prom/prometheus:v2.48.0`
- **Replicas**: 1 (single instance)
- **CPU/Memory**:
  - Request: 250m CPU, 512Mi RAM
  - Limit: 1000m CPU, 2Gi RAM
- **Key arguments**:
  - `--config.file=/etc/prometheus/prometheus.yml` - Points to config file
  - `--storage.tsdb.retention.time=7d` - Keeps 7 days of data
  - `--web.enable-lifecycle` - Allows config reload without restart
- **Port**: 9090
- **Health check**: HTTP probe on `/-/healthy`
- **Volumes**:
  - `config`: ConfigMap with prometheus.yml
  - `storage`: PVC for metric storage
  - `rules`: Alert rules directory

#### **configmap.yaml**
- **Name**: `prometheus-config`
- **Contains**: `prometheus.yml` configuration file
- **Key settings**:
  - `scrape_interval: 15s` - Collects metrics every 15 seconds
  - `evaluation_interval: 10s` - Evaluates alert rules every 10 seconds
  - `external_labels`: Tags metrics as `cluster: dev-cluster`, `environment: production`
- **Scrape targets** (what gets monitored):
  1. `kubernetes-apiservers` - Kubernetes API server
  2. `kubernetes-nodes` - Worker nodes
  3. `kubernetes-service-endpoints` - Services and pods
  4. `kubernetes-pods` - Pod metrics
  5. Custom jobs for internal services
- **AlertManager integration**: Routes alerts to `alertmanager:9093`

#### **services.yaml**
- **Kind**: Kubernetes Service
- **Type**: ClusterIP (internal only)
- **Port**: 9090
- **Purpose**: Exposes Prometheus to other services internally

#### **pvc.yaml**
- **Kind**: PersistentVolumeClaim
- **Size**: 10Gi (10 gigabytes)
- **Access mode**: ReadWriteOnce (one pod can write)
- **Purpose**: Stores time-series database files for Prometheus
- **Retention**: Data kept for 7 days, then deleted

#### **serviceaccounts.yaml**
- **ServiceAccount**: `prometheus`
- **RBAC Rules**: Permissions to read:
  - Pods, nodes, endpoints, services
  - Node proxy metrics
  - Service metrics
- **Purpose**: Allows Prometheus to discover and scrape Kubernetes resources

#### **local/**
**Alert rules for firing alerts**
- Contains `.yml` files with conditions like:
  - High CPU usage (>80%)
  - High memory usage (>85%)
  - Service unreachable
  - Error rate too high

---

## 📈 Grafana (`/grafana`)

**Beautiful dashboard for visualizing monitoring data**

### **Files**

#### **deployment.yaml**
- **Container**: Official Grafana image
- **Replicas**: 1
- **Key features**:
  - **Data Sources** configured in ConfigMap:
    1. **Prometheus** (http://prometheus:9090)
    2. **Google Cloud Monitoring** (StackDriver - queries GCP metrics)
    3. **Google Cloud Logging** (StackDriver - queries GCP logs)
  - **Dashboard provisioning**: Auto-loads dashboards from mounted ConfigMaps
  - **Admin credentials**: `admin:admin` (default)
- **Port**: 3000
- **Environment variables**: Database URL, security settings
- **Volumes**: Dashboards ConfigMap mounted

#### **dashboards/**

##### **k8s-monitoring.json**
- **Name**: Kubernetes Monitoring Dashboard
- **Purpose**: Shows cluster health
- **Visualizations** include:
  - CPU usage by node
  - Memory usage by pod
  - Network I/O
  - Pod status
  - Deployment health

##### **gke-dashboard.json**
- **Name**: GKE-Specific Dashboard
- **Purpose**: Google Kubernetes Engine specific metrics
- **Data from**: Google Cloud Monitoring API
- **Visualizations** include:
  - GKE node metrics
  - Container resource usage
  - Pod network metrics
  - Cluster health summary

---

## 🚨 AlertManager (`/alertmanager`)

**Intelligent alert routing and grouping**

### **Files**

#### **alertmanager-config.yaml**
- **Purpose**: Defines how alerts are routed to notification channels
- **Global settings**:
  - `resolve_timeout: 5m` - How long to wait before resolving alerts
- **Alert routing rules** (hierarchical):
  1. **Heartbeat alerts** → Main Discord channel (every 30 min)
  2. **Critical severity** → Discord (critical) + Phone call
  3. **Warning severity** → Discord (QA channel)
  4. **Default** → Default receiver
- **Features**:
  - `group_by`: Groups alerts by alertname, cluster, service
  - `group_wait: 30s`: Waits 30s before sending first alert (batches them)
  - `group_interval: 10s`: Waits 10s between sending grouped alerts
  - `repeat_interval: 12h`: Resends unresolved alerts every 12 hours
  - `continue: true/false`: Whether to continue to next route rule

#### **deployment.yaml**
- **Container**: `prom/alertmanager:latest`
- **Replicas**: 1
- **Port**: 9093
- **Config mounted**: AlertManager configuration from ConfigMap
- **Data volume**: Stores alert state

#### **service.yaml**
- **Type**: ClusterIP
- **Port**: 9093
- **Purpose**: Internal service for AlertManager access

---

## 📱 Discord Relay (`/discord-relay`)

**Converts alerts to Discord messages and triggers phone calls**

### **Files**

#### **main.go**
- **Language**: Go
- **Port**: 8080
- **Purpose**: Acts as webhook receiver from AlertManager
- **Key functions**:
  - Receives AlertManager webhook payloads
  - Converts alert JSON to Discord embeds (formatted messages)
  - Routes to different Discord channels based on severity
  - Can trigger phone calls for critical alerts
- **Endpoints**:
  - `/webhook/main` - General/heartbeat alerts
  - `/webhook/critical` - Critical incidents
  - `/webhook/warning` - Warning-level alerts
  - `/webhook/phone` - Triggers phone calls
- **Environment variables**:
  - `DISCORD_CRITICAL_URLS`: Webhook URLs for critical channel
  - `DISCORD_WARNING_URLS`: Webhook URLs for warning channel
  - `DISCORD_MAIN_URLS`: Webhook URLs for main channel
  - `DISCORD_PHONE_URL`: Webhook URL for phone integration

#### **Dockerfile**
- **Build stage**: Compiles Go code to binary
- **Runtime stage**: Alpine Linux (small, minimal)
- **User**: Runs as non-root `appuser` (security)
- **Output**: Executable `/app/discord-relay`

#### **deployment.yaml**
- **Container**: discord-relay:latest
- **Replicas**: 1
- **Environment** variables injected from ConfigMap
- **Port**: 8080

#### **service.yaml**
- **Type**: ClusterIP
- **Port**: 8080
- **Purpose**: Internal service for AlertManager to call

---

## 📊 Metrics Exporter (`/metrics-exporter`)

**Collects custom application metrics**

### **Files**

#### **main.go**
- **Language**: Go
- **Purpose**: Custom metrics collector for your applications
- **Port**: 8080
- **Metrics collected**:
  - **HTTP metrics**:
    - `http_requests_total` - Total requests by service/method/status
    - `http_request_duration_seconds` - Request latency histogram
  - **Auth metrics**:
    - `auth_failures` - Recent auth failures in last 5 minutes
  - **Queue metrics**:
    - `messages_in_queue` - Current messages in Pub/Sub
  - **Database metrics**:
    - Connection pool usage
    - Query latencies
  - **AI metrics**:
    - Tokens consumed
    - Model usage rates
- **Queries**: Connects to PostgreSQL to fetch real data
- **Exports**: Exposes metrics on `/metrics` endpoint in Prometheus format

#### **deployment.yaml**
- Container running metrics exporter
- Connects to PostgreSQL databases
- Port 8080

#### **Dockerfile**
- Build: Compiles Go app
- Runtime: Alpine Linux
- Non-root user

#### **go.mod**
- Dependencies:
  - PostgreSQL driver (`lib/pq`)
  - Prometheus client library
  - Database/sql standard library

---

## ☁️ GCP Exporter (`/gcp-exporter`)

**Pulls metrics from Google Cloud Platform**

### **Files**

#### **main.go**
- **Language**: Go
- **Purpose**: Queries Google Cloud Monitoring API
- **Metrics collected**:
  - **GKE Node metrics**:
    - `gke_node_cpu_utilization` - Node CPU %
    - `gke_node_memory_utilization` - Node memory %
    - `gke_node_disk_utilization` - Node disk %
  - **Container metrics**:
    - `gke_container_cpu_usage_seconds` - CPU time used
    - Container memory usage
  - **Pod metrics**:
    - Network I/O (bytes sent/received)
    - Pod status
  - **Cluster metrics**:
    - Overall health
    - Available resources
- **GCP API**: Uses `cloud.google.com/go/monitoring` SDK
- **Authentication**: Uses service account credentials (JSON key file)
- **Purpose**: Bridge between GCP infrastructure and Prometheus

#### **deployment.yaml**
- Container with GCP exporter
- Mounts GCP service account key file
- Port 8080

#### **Dockerfile**
- Multi-stage build
- Final image: Alpine Linux
- Non-root user

#### **go.mod**
- Dependencies:
  - Google Cloud monitoring library
  - Prometheus client library
  - gRPC and Protocol Buffers

#### **docker-compose.yml**
- Local development setup for GCP exporter

#### **cloudbuild.yaml**
- CI/CD configuration for Google Cloud Build
- Builds and pushes image to Container Registry

#### **setup.sh** / **setup.ps1**
- Bash/PowerShell scripts for initial setup
- Creates service account
- Sets up authentication
- Configures permissions

---

## 🖥️ System Exporters (`/exporters`)

**Collect infrastructure and Kubernetes metrics**

### **node-exporter.yaml**
- **Purpose**: Collects machine-level metrics from Kubernetes nodes
- **Metrics**:
  - CPU usage, temperature
  - Memory usage, page faults
  - Disk I/O, space usage
  - Network interface metrics
  - System uptime
- **Type**: DaemonSet (runs on every node)
- **RBAC**: ServiceAccount with permissions to read node data

### **kube-state-metrics.yaml**
- **Purpose**: Exports Kubernetes object metrics
- **Metrics**:
  - Pod status (running, pending, failed)
  - Deployment replicas (desired vs actual)
  - StatefulSet status
  - DaemonSet coverage
  - Job completion rates
  - PVC capacity
- **Type**: Deployment
- **RBAC**: Reads Kubernetes API objects

---

## 🔄 Data Flow

```
┌─────────────────────────────────────────────────┐
│         Application Services                    │
│  (Auth, Prompt Manager, API Gateway, etc)      │
└────────────┬────────────────────────────────────┘
             │ Expose /metrics endpoint
             ▼
┌─────────────────────────────────────────────────┐
│         System Exporters                        │
│  (Node Exporter, Kube-State Metrics)           │
└────────────┬────────────────────────────────────┘
             │
      ┌──────┴──────┐
      │             │
      ▼             ▼
┌──────────────┐  ┌──────────────────┐
│ Metrics      │  │ GCP Exporter     │
│ Exporter     │  │ (Cloud Monitoring)
└──────┬───────┘  └───────┬──────────┘
       │                  │
       └──────────┬───────┘
                  ▼
          ┌────────────────┐
          │  PROMETHEUS    │ (Every 15 seconds)
          │  (Time DB)     │
          └────────┬───────┘
                   │
      ┌────────────┼────────────┐
      ▼            ▼            ▼
 ┌─────────┐  ┌──────────┐  ┌────────────┐
 │ GRAFANA │  │ ALERT    │  │ Prometheus│
 │(Dashb.)│  │ RULES    │  │ Queries   │
 └────────┘  └────┬─────┘  └──────────┘
                  │
       ┌──────────▼──────────┐
       │   ALERTMANAGER     │
       │   (Route Alerts)   │
       └──────────┬─────────┘
             ┌────┴────┐
             ▼         ▼
      ┌──────────────┐ ┌──────────────┐
      │  Discord     │ │ Phone Call   │
      │  Relay       │ │ System       │
      └──────────────┘ └──────────────┘
             │                │
             └────────┬───────┘
                      ▼
              Team Notifications
```

---

## 🚀 Deployment

All components deploy together via:
```bash
kubectl kustomize monitoring/ | kubectl apply -f -
```

This applies the `kustomization.yaml` which:
1. Creates namespace `dev`
2. Deploys Prometheus + config + storage
3. Deploys Grafana + datasources + dashboards
4. Deploys AlertManager + rules
5. Deploys Discord Relay
6. Deploys all exporters
7. Injects environment variables (Discord URLs, etc.)

---

## 📝 Configuration Files Summary

| File | Type | Purpose |
|------|------|---------|
| `prometheus/configmap.yaml` | ConfigMap | Prometheus scrape targets & alert rules |
| `prometheus/deployment.yaml` | Deployment | Prometheus container & resources |
| `prometheus/pvc.yaml` | PVC | 10GB storage for metrics |
| `alertmanager/alertmanager-config.yaml` | ConfigMap | Alert routing rules |
| `grafana/deployment.yaml` | Deployment | Grafana with 3 datasources |
| `grafana/dashboards/*` | JSON | Pre-built visualization dashboards |
| `discord-relay/main.go` | Go Code | Alert to Discord conversion |
| `metrics-exporter/main.go` | Go Code | Custom app metrics collection |
| `gcp-exporter/main.go` | Go Code | GCP metrics collection |
| `exporters/*.yaml` | DaemonSet/Deployment | System metrics collection |

---

## 🔧 Common Operations

### View Prometheus UI
```
kubectl port-forward -n dev svc/prometheus 9090:9090
# Open: http://localhost:9090
```

### View Grafana Dashboards
```
kubectl port-forward -n dev svc/grafana 3000:3000
# Open: http://localhost:3000 (admin/admin)
```

### View AlertManager
```
kubectl port-forward -n dev svc/alertmanager 9093:9093
# Open: http://localhost:9093
```

### Query Prometheus Metrics
```bash
# Via port-forward to http://localhost:9090/api/v1/query
# Example: http_requests_total{service="auth"}
```

---

## ⚙️ Key Metrics Explained

- **CPU**: Measured in millicores (m) - 1000m = 1 core
- **Memory**: Measured in Mi (mebibytes) or Gi (gibibytes)
- **Requests**: Total count by endpoint
- **Latency**: Response time in seconds
- **Error rate**: Failed requests percentage
- **Uptime**: Service availability time