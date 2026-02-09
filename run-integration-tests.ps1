# PowerShell script to run integration tests for backend services

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Backend Integration Tests Runner" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if Docker is running
Write-Host "Checking Docker..." -ForegroundColor Yellow
$dockerRunning = docker ps 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Docker is not running. Please start Docker Desktop." -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Docker is running" -ForegroundColor Green
Write-Host ""

# Load environment variables from .env file
if (Test-Path .env) {
    Write-Host "Loading environment variables from .env..." -ForegroundColor Yellow
    Get-Content .env | ForEach-Object {
        if ($_ -match '^([^=]+)=(.*)$' -and $_ -notmatch '^#') {
            [Environment]::SetEnvironmentVariable($matches[1], $matches[2], "Process")
        }
    }
    Write-Host "[OK] Environment variables loaded" -ForegroundColor Green
} else {
    Write-Host "WARNING: .env file not found. Using test defaults." -ForegroundColor Yellow
    
    # Auth database variables (for docker-compose)
    $env:POSTGRES_HOST = "auth-db"
    $env:POSTGRES_PORT = "5432"
    $env:POSTGRES_USER = "postgres"
    $env:POSTGRES_PASSWORD = "postgres"
    $env:POSTGRES_DB = "devops_db"
    
    # Auth database variables (for Go tests)
    $env:DB_HOST = "localhost"
    $env:DB_PORT = "5432"
    $env:DB_USER = "postgres"
    $env:DB_PASSWORD = "postgres"
    $env:DB_NAME = "devops_db"
    $env:JWT_SECRET = "test-secret-key"
    
    # Chats database variables (for docker-compose and Go tests)
    $env:CHATS_POSTGRES_HOST = "chats_db"
    $env:CHATS_POSTGRES_PORT = "5432"
    $env:CHATS_POSTGRES_USER = "postgres"
    $env:CHATS_POSTGRES_PASSWORD = "postgres"
    $env:CHATS_POSTGRES_DB = "chats_db"
}
Write-Host ""

# Start required services for testing
Write-Host "Starting test services (databases, pub/sub, mock LLMs)..." -ForegroundColor Yellow
docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d auth-db chats_db pubsub-emulator gemma3 qwen3 auth
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Failed to start test services" -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Test services started" -ForegroundColor Green
Write-Host ""

# Wait for database to be ready
Write-Host "Waiting for auth database to be ready..." -ForegroundColor Yellow
$maxAttempts = 30
$attempt = 0
$dbReady = $false

while ($attempt -lt $maxAttempts -and -not $dbReady) {
    $attempt++
    # Use pg_isready inside the db container for robust health check
    docker-compose -f docker-compose.yml -f docker-compose.test.yml exec -T auth-db pg_isready -U $env:POSTGRES_USER -d $env:POSTGRES_DB 2>$null
    if ($LASTEXITCODE -eq 0) {
        $dbReady = $true
    } else {
        Start-Sleep -Seconds 1
        Write-Host "." -NoNewline
    }
}

Write-Host ""
if (-not $dbReady) {
    Write-Host "ERROR: Auth database did not become ready in time" -ForegroundColor Red
    docker-compose -f docker-compose.yml -f docker-compose.test.yml logs auth-db
    exit 1
}
Write-Host "[OK] Auth database is ready" -ForegroundColor Green
Write-Host ""

# Wait for chats database to be ready
Write-Host "Waiting for chats database to be ready..." -ForegroundColor Yellow
$attempt = 0
$chatsDbReady = $false

while ($attempt -lt $maxAttempts -and -not $chatsDbReady) {
    $attempt++
    docker-compose -f docker-compose.yml -f docker-compose.test.yml exec -T chats_db pg_isready -U $env:CHATS_POSTGRES_USER -d $env:CHATS_POSTGRES_DB 2>$null
    if ($LASTEXITCODE -eq 0) {
        $chatsDbReady = $true
    } else {
        Start-Sleep -Seconds 1
        Write-Host "." -NoNewline
    }
}

Write-Host ""
if (-not $chatsDbReady) {
    Write-Host "ERROR: Chats database did not become ready in time" -ForegroundColor Red
    docker-compose -f docker-compose.yml -f docker-compose.test.yml logs chats_db
    exit 1
}
Write-Host "[OK] Chats database is ready" -ForegroundColor Green
Write-Host ""

# Load test.env for auth service tests
$authTestEnvPath = "services\auth.service\test.env"
if (Test-Path $authTestEnvPath) {
    Write-Host "Loading auth service test environment variables..." -ForegroundColor Yellow
    Get-Content $authTestEnvPath | ForEach-Object {
        if ($_ -match '^([^=]+)=(.*)$' -and $_ -notmatch '^#') {
            [Environment]::SetEnvironmentVariable($matches[1], $matches[2], "Process")
        }
    }
    Write-Host "[OK] Auth test environment loaded" -ForegroundColor Green
} else {
    Write-Host "WARNING: test.env not found for auth service, using .env values" -ForegroundColor Yellow
    # Map POSTGRES_* variables to DB_* variables for tests
    $env:DB_HOST = "localhost"
    $env:DB_USER = $env:POSTGRES_USER
    $env:DB_PASSWORD = $env:POSTGRES_PASSWORD
    $env:DB_NAME = $env:POSTGRES_DB
    $env:DB_PORT = $env:POSTGRES_PORT
}
Write-Host ""

# Run Auth Service Tests
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Running Auth Service Integration Tests" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$authServicePath = Join-Path "services" "auth.service"
Push-Location $authServicePath
go test -v
$authTestResult = $LASTEXITCODE
Pop-Location

Write-Host ""

# Run API Gateway Tests
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Running API Gateway Integration Tests" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$gatewayServicePath = Join-Path "services" "api-gateway.service"
Push-Location $gatewayServicePath
go test -v
$gatewayTestResult = $LASTEXITCODE
Pop-Location

Write-Host ""

# Run Prompt Manager Tests
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Running Prompt Manager Integration Tests" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Set Prompt Manager specific environment variables
$env:DB_HOST = "localhost"
$env:DB_PORT = "5433"
$env:DB_USER = $env:CHATS_POSTGRES_USER
$env:DB_PASSWORD = $env:CHATS_POSTGRES_PASSWORD
$env:DB_NAME = $env:CHATS_POSTGRES_DB
$env:PUBSUB_EMULATOR_HOST = "localhost:8085"
$env:PUBSUB_PROJECT_ID = "local-project"
$env:PUBSUB_TOPIC_ID = "prompt-requests"
$env:PUBSUB_SUBSCRIPTION_ID = "prompt-requests-sub"
$env:AUTH_SERVICE_URL = "http://localhost:8001"
$env:GEMMA3_SERVICE_URL = "http://localhost:8003"
$env:QWEN3_SERVICE_URL = "http://localhost:8004"

$promptManagerServicePath = Join-Path "services" "prompt-manager.service"
Push-Location $promptManagerServicePath
go test -v
$promptManagerTestResult = $LASTEXITCODE
Pop-Location

Write-Host ""

# Summary
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Test Results Summary" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

if ($authTestResult -eq 0) {
    Write-Host "[PASS] Auth Service Tests: PASSED" -ForegroundColor Green
} else {
    Write-Host "[FAIL] Auth Service Tests: FAILED" -ForegroundColor Red
}

if ($gatewayTestResult -eq 0) {
    Write-Host "[PASS] API Gateway Tests: PASSED" -ForegroundColor Green
} else {
    Write-Host "[FAIL] API Gateway Tests: FAILED" -ForegroundColor Red
}

if ($promptManagerTestResult -eq 0) {
    Write-Host "[PASS] Prompt Manager Tests: PASSED" -ForegroundColor Green
} else {
    Write-Host "[FAIL] Prompt Manager Tests: FAILED" -ForegroundColor Red
}

Write-Host ""

# Clean up option
Write-Host "Do you want to stop the test services? (y/N): " -NoNewline -ForegroundColor Yellow
$response = Read-Host
if ($response -eq 'y' -or $response -eq 'Y') {
    Write-Host "Stopping test services..." -ForegroundColor Yellow
    docker-compose -f docker-compose.yml -f docker-compose.test.yml down
    Write-Host "[OK] Test services stopped" -ForegroundColor Green
}

# Exit with appropriate code
if ($authTestResult -ne 0 -or $gatewayTestResult -ne 0 -or $promptManagerTestResult -ne 0) {
    exit 1
}

exit 0
