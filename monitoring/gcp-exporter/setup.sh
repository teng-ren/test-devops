#!/bin/bash

# GCP Cloud Monitoring Exporter Setup Script
# This script sets up the GCP monitoring exporter with proper authentication

set -e

# Configuration
PROJECT_ID="${GCP_PROJECT_ID:-ai-model-tester-2}"
CLUSTER_NAME="${GKE_CLUSTER_NAME:-ai-model-tester-2}"
CLUSTER_REGION="${GKE_CLUSTER_REGION:-us-central1}"
NAMESPACE="dev"
KSA_NAME="gcp-exporter"
GSA_NAME="gcp-monitoring-exporter"

echo "================================================"
echo "GCP Cloud Monitoring Exporter Setup"
echo "================================================"
echo "Project ID: $PROJECT_ID"
echo "Cluster Name: $CLUSTER_NAME"
echo "Cluster Region: $CLUSTER_REGION"
echo "Namespace: $NAMESPACE"
echo "================================================"

# Step 1: Enable required APIs
echo "[1/8] Enabling required Google Cloud APIs..."
gcloud services enable monitoring.googleapis.com --project=$PROJECT_ID
gcloud services enable container.googleapis.com --project=$PROJECT_ID
echo "✓ APIs enabled"

# Step 2: Create GCP service account
echo "[2/8] Creating GCP service account..."
if gcloud iam service-accounts describe ${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --project=$PROJECT_ID &>/dev/null; then
  echo "  Service account already exists"
else
  gcloud iam service-accounts create $GSA_NAME \
    --display-name="GCP Monitoring Exporter Service Account" \
    --project=$PROJECT_ID
  echo "✓ Service account created"
fi

# Step 3: Grant Cloud Monitoring Viewer role
echo "[3/8] Granting Cloud Monitoring Viewer role..."
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/monitoring.viewer" \
  --condition=None \
  > /dev/null
echo "✓ Role granted"

# Step 4: Configure kubectl
echo "[4/8] Configuring kubectl context..."
gcloud container clusters get-credentials $CLUSTER_NAME \
  --region=$CLUSTER_REGION \
  --project=$PROJECT_ID
echo "✓ kubectl configured"

# Step 5: Create namespace if it doesn't exist
echo "[5/8] Creating namespace..."
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
echo "✓ Namespace ready"

# Step 6: Create Kubernetes service account
echo "[6/8] Creating Kubernetes service account..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $KSA_NAME
  namespace: $NAMESPACE
  annotations:
    iam.gke.io/gcp-service-account: ${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com
EOF
echo "✓ Kubernetes service account created"

# Step 7: Bind Kubernetes SA to GCP SA (Workload Identity)
echo "[7/8] Binding Workload Identity..."
gcloud iam service-accounts add-iam-policy-binding \
  ${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:${PROJECT_ID}.svc.id.goog[${NAMESPACE}/${KSA_NAME}]" \
  --project=$PROJECT_ID \
  > /dev/null
echo "✓ Workload Identity bound"

# Step 8: Build and deploy the exporter
echo "[8/8] Building and deploying GCP exporter..."

# Check if we should use Cloud Build or local build
if command -v gcloud &> /dev/null && [ -f "monitoring/gcp-exporter/cloudbuild.yaml" ]; then
  echo "  Using Cloud Build..."
  gcloud builds submit --config=monitoring/gcp-exporter/cloudbuild.yaml --project=$PROJECT_ID
else
  echo "  Using local Docker build..."
  cd monitoring/gcp-exporter
  docker build -t gcr.io/${PROJECT_ID}/gcp-exporter:latest .
  docker push gcr.io/${PROJECT_ID}/gcp-exporter:latest
  cd ../..
fi

# Apply Kubernetes manifests
echo "  Applying Kubernetes manifests..."
kubectl apply -f monitoring/gcp-exporter/deployment.yaml
kubectl apply -f monitoring/gcp-exporter/service.yaml

# Wait for deployment to be ready
echo "  Waiting for deployment to be ready..."
kubectl rollout status deployment/gcp-monitoring-exporter -n $NAMESPACE --timeout=120s

echo "✓ GCP exporter deployed"

echo ""
echo "================================================"
echo "Setup Complete!"
echo "================================================"
echo ""
echo "Verify the deployment:"
echo "  kubectl get pods -n $NAMESPACE -l app=gcp-monitoring-exporter"
echo ""
echo "Check logs:"
echo "  kubectl logs -n $NAMESPACE -l app=gcp-monitoring-exporter -f"
echo ""
echo "Test metrics endpoint:"
echo "  kubectl port-forward -n $NAMESPACE svc/gcp-exporter 8888:8888"
echo "  curl http://localhost:8888/metrics | grep gke_"
echo ""
echo "Update Prometheus to scrape the new exporter:"
echo "  kubectl apply -f monitoring/prometheus/configmap.yaml"
echo "  kubectl rollout restart deployment/prometheus -n $NAMESPACE"
echo ""
echo "Apply updated Grafana dashboard:"
echo "  # Upload monitoring/grafana/dashboards/k8s-monitoring.json via Grafana UI"
echo ""
