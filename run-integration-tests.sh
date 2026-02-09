#!/bin/bash
# Bash script to run integration tests for backend services
# Compatible with Linux/macOS and GitHub Actions

# Color codes for output
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Colo

echo -e "${CYAN}========================================"
echo -e "Backend Integration Tests Runner"
echo -e "========================================${NC}"
echo ""

# Check if Docker is running
echo -e "${YELLOW}Checking Docker...${NC}"
if ! docker ps > /dev/null 2>&1; then
    echo -e "${RED}ERROR: Docker is not running. Please start Docker.${NC}"
    exit 1
fi
echo -e "${GREEN}[OK] Docker is running${NC}"
echo ""

# Load environment variables from .env file
if [ -f .env ]; then
    echo -e "${YELLOW}Loading environment variables from .env...${NC}"
    # Export variables for docker-compose (same as PowerShell approach)
    export $(grep -v '^#' .env | grep -v '^[[:space:]]*$' | tr '\n' '\0' | xargs -0)
    echo -e "${GREEN}[OK] Environment variables loaded${NC}"
else
    echo -e "${YELLOW}WARNING: .env file not found. Using test defaults.${NC}"
    
    # Auth database variables (for docker-compose)
    export POSTGRES_HOST="auth-db"
    export POSTGRES_PORT="5432"
    export POSTGRES_USER="postgres"
    export POSTGRES_PASSWORD="postgres"
    export POSTGRES_DB="devops_db"
    
    # Auth database variables (for Go tests)
    export DB_HOST="localhost"
    export DB_PORT="5432"
    export DB_USER="postgres"
    export DB_PASSWORD="postgres"
    export DB_NAME="devops_db"
    export JWT_SECRET="test-secret-key"
    
    # Chats database variables (for docker-compose and Go tests)
    export CHATS_POSTGRES_HOST="chats_db"
    export CHATS_POSTGRES_PORT="5432"
    export CHATS_POSTGRES_USER="postgres"
    export CHATS_POSTGRES_PASSWORD="postgres"
    export CHATS_POSTGRES_DB="chats_db"
fi
echo ""

# Ensure CHATS_POSTGRES_* variables have defaults even if .env exists but doesn't define them
export CHATS_POSTGRES_HOST="${CHATS_POSTGRES_HOST:-chats_db}"
export CHATS_POSTGRES_PORT="${CHATS_POSTGRES_PORT:-5432}"
export CHATS_POSTGRES_USER="${CHATS_POSTGRES_USER:-postgres}"
export CHATS_POSTGRES_PASSWORD="${CHATS_POSTGRES_PASSWORD:-postgres}"
export CHATS_POSTGRES_DB="${CHATS_POSTGRES_DB:-chats_db}"

# Ensure POSTGRES_* variables have defaults (for auth DB)
export POSTGRES_HOST="${POSTGRES_HOST:-auth-db}"
export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
export POSTGRES_DB="${POSTGRES_DB:-devops_db}"

# Start required services for testing
echo -e "${YELLOW}Starting test services (databases, pub/sub, mock LLMs)...${NC}"
if ! docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d auth-db chats_db pubsub-emulator gemma3 qwen3 auth; then
    echo -e "${RED}ERROR: Failed to start test services${NC}"
    exit 1
fi
echo -e "${GREEN}[OK] Test services started${NC}"
echo ""

# Wait for database to be ready
echo -e "${YELLOW}Waiting for auth database to be ready...${NC}"
max_attempts=30
attempt=0
db_ready=false

while [ $attempt -lt $max_attempts ] && [ "$db_ready" = false ]; do
    attempt=$((attempt + 1))
    # Use pg_isready inside the db container for robust health check
    if docker-compose -f docker-compose.yml -f docker-compose.test.yml exec -T auth-db pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" > /dev/null 2>&1; then
        db_ready=true
    else
        sleep 1
        echo -n "."
    fi
done

echo ""
if [ "$db_ready" = false ]; then
    echo -e "${RED}ERROR: Auth database did not become ready in time${NC}"
    docker-compose -f docker-compose.yml -f docker-compose.test.yml logs auth-db
    exit 1
fi
echo -e "${GREEN}[OK] Auth database is ready${NC}"
echo ""

# Wait for chats database to be ready
echo -e "${YELLOW}Waiting for chats database to be ready...${NC}"
attempt=0
chats_db_ready=false

while [ $attempt -lt $max_attempts ] && [ "$chats_db_ready" = false ]; do
    attempt=$((attempt + 1))
    if docker-compose -f docker-compose.yml -f docker-compose.test.yml exec -T chats_db pg_isready -U "$CHATS_POSTGRES_USER" -d "$CHATS_POSTGRES_DB" > /dev/null 2>&1; then
        chats_db_ready=true
    else
        sleep 1
        echo -n "."
    fi
done

echo ""
if [ "$chats_db_ready" = false ]; then
    echo -e "${RED}ERROR: Chats database did not become ready in time${NC}"
    docker-compose -f docker-compose.yml -f docker-compose.test.yml logs chats_db
    exit 1
fi
echo -e "${GREEN}[OK] Chats database is ready${NC}"
echo ""

# Load test.env for auth service tests
if [ -f "services/auth.service/test.env" ]; then
    echo -e "${YELLOW}Loading auth service test environment variables...${NC}"
    export $(grep -v '^#' services/auth.service/test.env | grep -v '^[[:space:]]*$' | tr '\n' '\0' | xargs -0)
    echo -e "${GREEN}[OK] Auth test environment loaded${NC}"
else
    echo -e "${YELLOW}WARNING: test.env not found for auth service, using .env values${NC}"
    # Map POSTGRES_* variables to DB_* variables for tests
    export DB_HOST="localhost"
    export DB_USER="$POSTGRES_USER"
    export DB_PASSWORD="$POSTGRES_PASSWORD"
    export DB_NAME="$POSTGRES_DB"
    export DB_PORT="$POSTGRES_PORT"
fi
echo ""

# Run Auth Service Tests
echo -e "${CYAN}========================================"
echo -e "Running Auth Service Integration Tests"
echo -e "========================================${NC}"
echo ""

cd services/auth.service || exit 1
go test -v
auth_test_result=$?
cd ../.. || exit 1

echo ""

# Run API Gateway Tests
echo -e "${CYAN}========================================"
echo -e "Running API Gateway Integration Tests"
echo -e "========================================${NC}"
echo ""

cd services/api-gateway.service || exit 1
go test -v
gateway_test_result=$?
cd ../.. || exit 1

echo ""

# Run Prompt Manager Tests
echo -e "${CYAN}========================================"
echo -e "Running Prompt Manager Integration Tests"
echo -e "========================================${NC}"
echo ""

# Set Prompt Manager specific environment variables
export DB_HOST="localhost"
export DB_PORT="5433"
export DB_USER="$CHATS_POSTGRES_USER"
export DB_PASSWORD="$CHATS_POSTGRES_PASSWORD"
export DB_NAME="$CHATS_POSTGRES_DB"
export PUBSUB_EMULATOR_HOST="localhost:8085"
export PUBSUB_PROJECT_ID="local-project"
export PUBSUB_TOPIC_ID="prompt-requests"
export PUBSUB_SUBSCRIPTION_ID="prompt-requests-sub"
export AUTH_SERVICE_URL="http://localhost:8001"
export GEMMA3_SERVICE_URL="http://localhost:8003"
export QWEN3_SERVICE_URL="http://localhost:8004"

cd services/prompt-manager.service || exit 1
go test -v
prompt_manager_test_result=$?
cd ../.. || exit 1

echo ""

# Summary
echo -e "${CYAN}========================================"
echo -e "Test Results Summary"
echo -e "========================================${NC}"

if [ $auth_test_result -eq 0 ]; then
    echo -e "${GREEN}[PASS] Auth Service Tests: PASSED${NC}"
else
    echo -e "${RED}[FAIL] Auth Service Tests: FAILED${NC}"
fi

if [ $gateway_test_result -eq 0 ]; then
    echo -e "${GREEN}[PASS] API Gateway Tests: PASSED${NC}"
else
    echo -e "${RED}[FAIL] API Gateway Tests: FAILED${NC}"
fi

if [ $prompt_manager_test_result -eq 0 ]; then
    echo -e "${GREEN}[PASS] Prompt Manager Tests: PASSED${NC}"
else
    echo -e "${RED}[FAIL] Prompt Manager Tests: FAILED${NC}"
fi

echo ""

# Clean up option
echo -en "${YELLOW}Do you want to stop the test services? (y/N): ${NC}"
read -r response
if [[ "$response" =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Stopping test services...${NC}"
    docker-compose -f docker-compose.yml -f docker-compose.test.yml down
    echo -e "${GREEN}[OK] Test services stopped${NC}"
fi

# Exit with appropriate code
if [ $auth_test_result -ne 0 ] || [ $gateway_test_result -ne 0 ] || [ $prompt_manager_test_result -ne 0 ]; then
    exit 1
fi

exit 0
