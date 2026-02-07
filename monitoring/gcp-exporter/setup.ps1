# GCP Cloud Monitoring Exporter Setup Script (PowerShell)
# This script sets up the GCP monitoring exporter with proper authentication

param(
    [string]$ProjectId = "dop-assignment-team1",
    [string]$ClusterName = "ai-model-tester-2",
    [string]$ClusterRegion = "us-central1-a",
    [string]$Namespace = "dev"
)

$ErrorActionPreference = "Stop"

$KSA_NAME = "gcp-exporter"
$GSA_NAME = "gcp-monitoring-exporter"

Write-Host "================================================" -ForegroundColor Cyan
Write-Host "GCP Cloud Monitoring Exporter Setup" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "Project ID: $ProjectId"
Write-Host "Cluster Name: $ClusterName"
Write-Host "Cluster Region: $ClusterRegion"
Write-Host "Namespace: $Namespace"
Write-Host "================================================" -ForegroundColor Cyan

# Step 1: Enable required APIs
Write-Host "`n[1/8] Enabling required Google Cloud APIs..." -ForegroundColor Yellow
gcloud services enable monitoring.googleapis.com --project=$ProjectId
gcloud services enable container.googleapis.com --project=$ProjectId
Write-Host "[OK] APIs enabled" -ForegroundColor Green

# Step 2: Create GCP service account
Write-Host "`n[2/8] Creating GCP service account..." -ForegroundColor Yellow
try {
    $saExists = gcloud iam service-accounts describe "${GSA_NAME}@${ProjectId}.iam.gserviceaccount.com" --project=$ProjectId 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  Service account already exists" -ForegroundColor Gray
    } else {
        throw "Not found"
    }
} catch {
    gcloud iam service-accounts create $GSA_NAME `
        --display-name="GCP Monitoring Exporter Service Account" `
        --project=$ProjectId
    Write-Host "[OK] Service account created" -ForegroundColor Green
}

# Step 3: Grant Cloud Monitoring Viewer role
Write-Host "`n[3/8] Granting Cloud Monitoring Viewer role..." -ForegroundColor Yellow
gcloud projects add-iam-policy-binding $ProjectId `
    --member="serviceAccount:${GSA_NAME}@${ProjectId}.iam.gserviceaccount.com" `
    --role="roles/monitoring.viewer" `
    --condition=None `
    --quiet
Write-Host "[OK] Role granted" -ForegroundColor Green

# Step 4: Configure kubectl
Write-Host "`n[4/8] Configuring kubectl context..." -ForegroundColor Yellow
gcloud container clusters get-credentials $ClusterName `
    --region=$ClusterRegion `
    --project=$ProjectId
Write-Host "[OK] kubectl configured" -ForegroundColor Green

# Step 5: Create namespace if it doesn't exist
Write-Host "`n[5/8] Creating namespace..." -ForegroundColor Yellow
kubectl create namespace $Namespace --dry-run=client -o yaml | kubectl apply -f -
Write-Host "[OK] Namespace ready" -ForegroundColor Green

# Step 6: Create Kubernetes service account
Write-Host "`n[6/8] Creating Kubernetes service account..." -ForegroundColor Yellow
$ksaYaml = @"
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $KSA_NAME
  namespace: $Namespace
  annotations:
    iam.gke.io/gcp-service-account: ${GSA_NAME}@${ProjectId}.iam.gserviceaccount.com
"@
$ksaYaml | kubectl apply -f -
Write-Host "[OK] Kubernetes service account created" -ForegroundColor Green

# Step 7: Bind Kubernetes SA to GCP SA (Workload Identity)
Write-Host "`n[7/8] Binding Workload Identity..." -ForegroundColor Yellow
gcloud iam service-accounts add-iam-policy-binding `
    "${GSA_NAME}@${ProjectId}.iam.gserviceaccount.com" `
    --role roles/iam.workloadIdentityUser `
    --member "serviceAccount:${ProjectId}.svc.id.goog[${Namespace}/${KSA_NAME}]" `
    --project=$ProjectId `
    --quiet
Write-Host "[OK] Workload Identity bound" -ForegroundColor Green

# Step 8: Build and deploy the exporter
Write-Host "`n[8/8] Building and deploying GCP exporter..." -ForegroundColor Yellow

# Check if Cloud Build config exists
if (Test-Path "cloudbuild.yaml") {
    Write-Host "  Using Cloud Build..." -ForegroundColor Gray
    gcloud builds submit --config=cloudbuild.yaml --project=$ProjectId
} elseif (Test-Path "..\..\monitoring\gcp-exporter\cloudbuild.yaml") {
    Write-Host "  Using Cloud Build (from root)..." -ForegroundColor Gray
    Push-Location ..\..\monitoring\gcp-exporter
    gcloud builds submit --config=cloudbuild.yaml --project=$ProjectId
    Pop-Location
} else {
    Write-Host "  Cloud Build config not found. Skipping build (image already exists)." -ForegroundColor Yellow
}

# Apply Kubernetes manifests
Write-Host "  Applying Kubernetes manifests..." -ForegroundColor Gray
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Wait for deployment to be ready
Write-Host "  Waiting for deployment to be ready..." -ForegroundColor Gray
kubectl rollout status deployment/gcp-monitoring-exporter -n $Namespace --timeout=120s

Write-Host "[OK] GCP exporter deployed" -ForegroundColor Green

Write-Host "`n================================================" -ForegroundColor Cyan
Write-Host "Setup Complete!" -ForegroundColor Green
Write-Host "================================================" -ForegroundColor Cyan

Write-Host "`nVerify the deployment:" -ForegroundColor Yellow
Write-Host "  kubectl get pods -n $Namespace -l app=gcp-monitoring-exporter"

Write-Host "`nCheck logs:" -ForegroundColor Yellow
Write-Host "  kubectl logs -n $Namespace -l app=gcp-monitoring-exporter -f"

Write-Host "`nTest metrics endpoint:" -ForegroundColor Yellow
Write-Host "  kubectl port-forward -n $Namespace svc/gcp-exporter 8888:8888"
Write-Host "  curl http://localhost:8888/metrics | Select-String 'gke_'"

Write-Host "`nUpdate Prometheus to scrape the new exporter:" -ForegroundColor Yellow
Write-Host "  kubectl apply -f monitoring/prometheus/configmap.yaml"
Write-Host "  kubectl rollout restart deployment/prometheus -n $Namespace"

Write-Host "`nApply updated Grafana dashboard:" -ForegroundColor Yellow
Write-Host "  # Upload monitoring/grafana/dashboards/k8s-monitoring.json via Grafana UI"
Write-Host "  # or restart Grafana to pick up changes"
Write-Host ""
