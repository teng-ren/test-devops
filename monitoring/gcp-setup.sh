#!/bin/bash
# GCP Service Account Setup for Grafana Cloud Monitoring Integration
# Project: dop-assignment-team1
# Cluster: ai-model-tester-2 (us-central1-a)

PROJECT_ID="dop-assignment-team1"
SERVICE_ACCOUNT="grafana-monitoring"
NAMESPACE="dev"

echo "Setting up GCP service account for Grafana Cloud Monitoring..."

# Create service account
gcloud iam service-accounts create $SERVICE_ACCOUNT \
  --display-name="Grafana Cloud Monitoring Service Account" \
  --project=$PROJECT_ID 2>/dev/null || echo "Service account already exists"

# Grant necessary roles
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/monitoring.metricReader \
  --quiet

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/logging.logWriter \
  --quiet

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com \
  --role=roles/logging.viewer \
  --quiet

# Create and download JSON key
gcloud iam service-accounts keys create /tmp/grafana-sa-key.json \
  --iam-account=${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com \
  --project=$PROJECT_ID 2>/dev/null || echo "Key creation may have failed"

# Create Kubernetes secret from JSON key
kubectl create secret generic gcp-service-account \
  --from-file=key.json=/tmp/grafana-sa-key.json \
  -n $NAMESPACE \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Setup complete! Secret 'gcp-service-account' created in namespace '$NAMESPACE'"

# Cleanup local key
rm -f /tmp/grafana-sa-key.json
