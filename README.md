# DevOps_Oct2025_Team1_Assignment

##  Project Overview

This is a **complete cloud application system** that runs AI chatbots, manages user authentication, and monitors everything to keep the service running smoothly. Think of it as:
-  A platform where users can chat with AI
-  A security system that verifies who users are
-  A monitoring dashboard showing system health
-  An alert system that warns when something goes wrong

The application is designed to run on **Kubernetes** (a container orchestration platform - essentially a smart machine manager) in **Google Cloud** (GCP).

---

##  System Architecture

### **High-Level Overview**

\\\

                     USERS                               
         (Web Browser or Mobile App)                     

                 
                 

                    FRONTEND (Web UI)                    
  React App - Beautiful interface for users to interact  
           Deployed in Docker container                 

                 
                 

              BACKEND API GATEWAY                        
  Routes requests to the right microservices            

         
                                                 
        
      AUTH       PROMPT       LLM     MONITORING  
    SERVICE     MANAGER     SERVICE    SYSTEM     
        
\\\

---

##  Main Components

### **1. Frontend Application (\/app\)**
**The interface users see and interact with**

- **Technology**: React (JavaScript framework for building web interfaces)
- **What it does**:
  - Provides a beautiful website/web app
  - Users log in here
  - Users chat with AI here
  - Displays dashboards (for admin users)
- **Files**:
  - \src/\ - React components (building blocks of the interface)
  - \Dockerfile\ - Recipe to package app in a container
  - \package.json\ - List of software dependencies
- **How it works**: Browser downloads the app, renders HTML/CSS/JavaScript that creates the interface

---

### **2. Backend Services (\/services\)**
**The brain of the operation - these handle all the logic**

#### **2a. Authentication Service** (\uth.service/\)
**Keeps users safe by verifying who they are**

- **Purpose**: User login/registration, password security, token generation
- **Database**: PostgreSQL (stores user accounts securely)
- **Key features**:
  - Hashes passwords (scrambles them so admins can't see them)
  - Generates JWT tokens (digital ID cards that prove who the user is)
  - Manages user roles (admin, user, etc.)
- **Files**:
  - \main.go\ - Entry point
  - \uth-handlers.go\ - Login/register logic
  - \dmin-handlers.go\ - Admin-only operations
  - \db/\ - Database migration scripts

#### **2b. API Gateway** (\pi-gateway.service/\)
**The traffic director - routes requests to the right service**

- **Purpose**: Acts like a reception desk, sends requests to correct departments
- **Key features**:
  - Validates incoming requests
  - Checks authentication tokens
  - Routes to appropriate service
  - Handles load balancing if multiple services are running
- **Language**: Go (fast, efficient)

#### **2c. Prompt Manager Service** (\prompt-manager.service/\)
**Manages AI conversation history and settings**

- **Purpose**: Stores and retrieves chat conversations
- **Database**: PostgreSQL
- **Key features**:
  - Creates new chat sessions
  - Stores messages from both user and AI
  - Publishes events to message queue (Pub/Sub)
  - Tracks token usage (cost of running AI)
- **Files**:
  - \main.go\ - Entry point
  - \handlers.go\ - HTTP endpoints (API methods)
  - \models.go\ - Data structures
  - \pubsub.go\ - Message publishing logic
  - \worker.go\ - Background jobs

#### **2d. LLM Service** (\llm.service/\)
**Communicates with AI models**

- **Purpose**: Sends prompts to AI models (Gemma3, Qwen3) and gets responses
- **Key features**:
  - Accepts prompts from Prompt Manager
  - Calls LLM (Large Language Model) APIs
  - Returns generated text
  - Streams responses back

---

### **3. Monitoring & Observability (\/monitoring\)**
**Watches the entire system and alerts when problems occur**

#### **Core Monitoring Stack**

**Prometheus** (\prometheus/\)
- **What**: Time-series database that stores metrics (numbers over time)
- **Purpose**: Collects health data from all services
- **Metrics collected**:
  - CPU usage (how hard the server is working)
  - Memory usage (RAM being used)
  - HTTP requests (how many API calls)
  - Database connections
- **Data retention**: 7 days of historical data
- **Files**:
  - \configmap.yaml\ - Configuration for scraping targets
  - \deployment.yaml\ - Kubernetes deployment instructions
  - \local/\ - Alert rules (conditions for firing alerts)

**Grafana** (\grafana/\)
- **What**: Beautiful dashboard to visualize data
- **Purpose**: Displays metrics and system health in charts/graphs
- **Features**:
  - Real-time graphs of CPU, memory, requests
  - Pre-built dashboards for Kubernetes and GKE
  - Connected to 3 data sources:
    1. Local Prometheus (your application metrics)
    2. Google Cloud Monitoring (GCP infrastructure)
    3. Google Cloud Logging (Application logs)
- **Access**: Open web browser to see dashboards

**AlertManager** (\lertmanager/\)
- **What**: Intelligent alert routing system
- **Purpose**: Decides what alert to send where based on severity
- **Routing logic**:
  - **Critical alerts**  Discord (dev team) + Phone call
  - **Warning alerts**  Discord (QA team)
  - **Heartbeat**  Main channel (every 30 min, proves system is alive)
- **Features**:
  - Groups similar alerts
  - Prevents alert bombardment
  - Escalates critical issues

**Discord Relay** (\discord-relay/\)
- **What**: Bridge between AlertManager and Discord
- **Purpose**: Converts alerts into Discord messages and forwards them
- **Features**:
  - Formats alerts with color codes (red = critical, yellow = warning)
  - Sends to different Discord channels
  - Can trigger phone calls for critical issues
  - Includes relevant details (server name, error message)

#### **Data Collection (Exporters)**

**Metrics Exporter** (\metrics-exporter/\)
- **What**: Custom app that gathers application-specific metrics
- **Data collected**:
  - HTTP request counts and latencies
  - Authentication failure rates
  - Message queue depth
  - Database connection pool usage
  - Token consumption
- **How**: Queries your databases and exposes metrics in Prometheus format

**GCP Exporter** (\gcp-exporter/\)
- **What**: Custom app that pulls metrics from Google Cloud
- **Data collected**:
  - Kubernetes node CPU/memory/disk usage
  - Container resource usage
  - Pod network I/O
  - GKE cluster health
- **How**: Uses Google Cloud Monitoring API

**System Exporters** (\exporters/\)
- **Node Exporter**: Collects machine-level metrics (CPU, disk, network)
- **Kube-State Metrics**: Kubernetes cluster status (pod count, deployment health)

---

### **4. Container Orchestration & Local Setup**

#### **Docker** (\Dockerfile\)
- **What**: Technology that packages applications into "containers"
- **Purpose**: Ensures app works the same everywhere (laptop, cloud, CI/CD)
- **How**: Creates a box with app + all dependencies
- **Benefits**: "Works on my machine" problem solved!

#### **Kubernetes** (K8s) - Production
- **What**: Container orchestration system
- **Purpose**: Manages, scales, and heals containerized applications
- **Key features**:
  - Auto-restarts failed services
  - Scales services up/down based on demand
  - Load balances traffic
  - Handles storage and networking
- **Files**: Various \.yaml\ files defining deployments

#### **Docker Compose** - Local Development
- **What**: Tool to run multiple containers locally
- **Purpose**: Simulate full stack on your laptop
- **Files**:
  - \docker-compose.yml\ - Production-like setup
  - \docker-compose.test.yml\ - For running tests

#### **Kubernetes Local Setup** (\k8s-local/\)
- **Mock LLM Service**: Simulates AI model for local development
- **Pub/Sub Emulator**: Simulates Google Cloud Pub/Sub for messages
- **Allows**: Testing without connecting to real GCP

---

##  Data Flow - How Everything Works Together

### **User Login Flow**
\\\
User types username/password in frontend
        
Frontend sends to API Gateway
        
API Gateway routes to Auth Service
        
Auth Service checks database
        
Auth Service returns JWT token (digital ID)
        
Frontend stores token and shows dashboard
\\\

### **User Sends Chat Message Flow**
\\\
User types message in chat interface
        
Frontend sends to API Gateway + JWT token
        
API Gateway validates token with Auth Service
        
Routes to Prompt Manager Service
        
Prompt Manager stores message in database
        
Publishes "new message" event to Pub/Sub queue
        
LLM Service picks up event from queue
        
LLM Service calls AI model API (Gemma3/Qwen3)
        
AI model returns response
        
LLM Service stores response in database
        
Frontend polls or receives response (WebSocket)
        
User sees AI's answer
\\\

### **Monitoring & Alerting Flow**
\\\
All Services expose metrics (CPU, requests, errors)
        
Prometheus scrapes metrics every 15 seconds
        
Prometheus stores in time-series database
        
AlertManager evaluates alert rules:
  - Is CPU > 80%?
  - Are errors > threshold?
  - Is service down?
        
If alert triggered:
  - If CRITICAL  AlertManager sends to Discord Relay
  - Discord Relay posts to Discord channels
  - Discord Relay calls phone system
        
Team gets notified immediately
        
Team logs into Grafana dashboard to investigate
        
Sees graphs of what happened
\\\

---

##  Deployment Architecture

### **Development Environment**
- Run locally with Docker Compose
- Uses mock LLM and Pub/Sub
- Database: PostgreSQL containers

### **Google Kubernetes Engine (GKE)**
- **What**: Google's managed Kubernetes service
- **How it works**:
  1. Code pushed to GitHub
  2. Cloud Build automatically builds Docker images
  3. Images pushed to Google Container Registry
  4. Deployed to GKE cluster
  5. Kustomize applies configuration
- **Auto-scaling**: Services scale based on CPU/memory usage
- **Load Balancing**: Traffic distributed across replicas

### **GCP Services Used**
- **Kubernetes Engine (GKE)**: Runs containers
- **Cloud Pub/Sub**: Message queue for async processing
- **Cloud SQL**: Managed PostgreSQL database
- **Cloud Monitoring**: Infrastructure metrics
- **Cloud Logging**: Application logs
- **Container Registry**: Image storage
- **Cloud Build**: CI/CD pipeline

---

##  Monitoring & Alerts in Plain English

### **What Gets Monitored**
-  Server CPU usage (is it overworked?)
-  Memory usage (is storage full?)
-  HTTP request count (how much traffic?)
-  Request latency (are responses slow?)
-  Error rates (are services failing?)
-  Database connections (is database healthy?)
-  Queue depth (are messages backing up?)
-  Authentication failures (are there attacks?)
-  AI token consumption (are we spending money fast?)

### **Alert Severities**
-  **CRITICAL**: Service down, database unreachable  Phone call + Discord
-  **WARNING**: High error rate, slow responses  Discord notification
-  **INFO**: Heartbeat, regular updates  Quiet logging

---

##  Local Development

### **Quick Start**
\\\ash
# Clone repo
git clone <repo-url>
cd test

# Start services with Docker Compose
docker-compose up

# Services available at:
# - Frontend: http://localhost:3000
# - API Gateway: http://localhost:8000
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3001 (admin/admin)
\\\

### **Files Structure Explained**
\\\
test/
 app/                    # React frontend + Vite build config
 services/              # Go microservices
    auth.service/      # User authentication
    api-gateway.service/   # Request router
    prompt-manager.service/   # Chat/prompt storage
    llm.service/       # AI model integration
 monitoring/            # Prometheus, Grafana, Alerts
    prometheus/        # Metrics database
    grafana/           # Dashboards
    alertmanager/      # Alert routing
    discord-relay/     # Notification bridge
    metrics-exporter/  # App metrics collection
    gcp-exporter/      # GCP metrics collection
    exporters/         # System metrics
 k8s-local/             # Local Kubernetes config for testing
 docker-compose.yml     # Production-like local setup
 docker-compose.test.yml # Testing setup
\\\

---

##  Key Technologies Explained for Beginners

| Technology | What It Is | Why We Use It |
|-----------|-----------|---------------|
| **React** | JavaScript library for building web interfaces | Create beautiful, interactive websites |
| **Go** | Fast programming language | Backend services run quickly, handle many requests |
| **PostgreSQL** | Database for storing structured data | Reliable, proven, excellent for applications |
| **Kubernetes** | Container orchestration platform | Manage hundreds of containers automatically |
| **Docker** | Containerization platform | Package apps to work anywhere |
| **Prometheus** | Time-series database | Store metrics over time for trend analysis |
| **Grafana** | Data visualization tool | Create beautiful charts and dashboards |
| **AlertManager** | Alert management system | Route alerts intelligently based on rules |
| **Pub/Sub** | Message queue system | Decouple services, handle async processing |
| **GCP** | Google Cloud Platform | Reliable cloud infrastructure |

---

##  Security Features

- **Password Hashing**: Passwords never stored in plain text
- **JWT Tokens**: Stateless authentication system
- **HTTPS/TLS**: Encrypted communication between services
- **RBAC**: Role-based access control (admin, user, guest)
- **Service Accounts**: Services authenticate to each other securely
- **Network Policies**: Kubernetes restricts service-to-service communication

---

##  Scalability

- **Horizontal Scaling**: Add more pods (containers) when traffic increases
- **Load Balancing**: Traffic distributed across multiple instances
- **Database Scaling**: Cloud SQL handles read replicas
- **Auto-scaling Rules**: Metrics-based scaling (CPU/memory thresholds)
- **Caching**: Reduce database queries
- **Rate Limiting**: Prevent abuse

---

##  Troubleshooting Guide

### **Service Not Responding**
1. Check Prometheus - is container running?
2. Check logs in Grafana/Cloud Logging
3. Check network policies and firewalls

### **High Error Rate**
1. Check AlertManager alerts
2. Look at Grafana dashboards
3. Check service logs
4. Verify database connectivity

### **Slow Requests**
1. Check CPU/memory usage
2. Run database query analysis
3. Check network latency
4. Review application logs

---

##  Learning Resources

- **Kubernetes Basics**: https://kubernetes.io/docs/tutorials/
- **Docker Concepts**: https://docs.docker.com/get-started/
- **Prometheus Metrics**: https://prometheus.io/docs/
- **Grafana Dashboards**: https://grafana.com/docs/
- **Google Cloud**: https://cloud.google.com/docs

---

##  Team & Support

This project was created by Team 1 as part of the DevOps assignment (October 2025).

For questions or issues:
1. Check the monitoring dashboards (Prometheus/Grafana)
2. Review the alert messages in Discord
3. Check service logs
4. Review code documentation in individual services

---

##  Summary

This is a **complete, production-ready AI chat application** with:
-  Secure user authentication
-  AI-powered chat interface
-  Real-time monitoring of all components
-  Intelligent alerting system
-  Beautiful dashboards for understanding system health
-  Automatic scaling and self-healing

Everything works together in a **microservices architecture**, where each component has a specific job, making the system scalable, reliable, and easy to maintain.
