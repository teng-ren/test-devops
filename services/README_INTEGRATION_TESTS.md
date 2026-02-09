# Integration Testing Guide

This document explains how to run integration tests for the backend Go services.

## Overview

Integration tests have been created for:
- **Auth Service** (`auth.service/auth_integration_test.go`)
- **API Gateway** (`api-gateway.service/gateway_integration_test.go`)
- **Prompt Manager Service** (`prompt-manager.service/prompt_manager_integration_test.go`)

## Prerequisites

1. **Docker and Docker Compose** installed
2. **Go 1.25+** installed
3. **PostgreSQL databases** running (via Docker Compose)
4. **Pub/Sub Emulator** running (for prompt-manager tests)
5. **Mock LLM Services** running (for prompt-manager tests)

## Running Integration Tests

### Quick Start (Recommended)

Use the automated test runner scripts that handle all service setup and teardown:

**Windows PowerShell:**
```powershell
.\run-integration-tests.ps1
```

**Linux/macOS/WSL:**
```bash
./run-integration-tests.sh
```

These scripts will:
1. Start all required services (databases, Pub/Sub emulator, mock LLMs, auth service)
2. Wait for services to be ready
3. Run all integration tests (Auth, API Gateway, Prompt Manager)
4. Display a summary of test results
5. Optionally clean up services

### Manual Testing

If you prefer to run tests manually:

### Step 1: Start Required Services

Start all test services using Docker Compose:

```bash
docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d auth-db chats_db pubsub-emulator gemma3 qwen3 auth
```

Wait for services to be healthy (about 10-15 seconds):

```bash
docker-compose ps
```

### Step 2: Set Environment Variables

For Auth Service tests, set the required environment variables.

**Option A: Create test.env file from template**

First, copy the example file and update the placeholder values:

```bash
# Copy the template
cp services/auth.service/test.env.example services/auth.service/test.env

# Edit test.env and replace the placeholder values:
# - DB_USER (change from "test_user_CHANGE_ME")
# - DB_PASSWORD (change from "CHANGE_ME_test_password_min_16_chars")
# - JWT_SECRET (change from "CHANGE_ME_random_secret_min_32_chars_for_testing_only")
```

Then source the file:

```bash
# Linux/Mac
source services/auth.service/test.env

# Windows (PowerShell)
Get-Content services\auth.service\test.env | ForEach-Object {
    if ($_ -match '^([^=]+)=(.*)$') {
        [Environment]::SetEnvironmentVariable($matches[1], $matches[2])
    }
}
```

**Option B: Set environment variables manually**

**Windows PowerShell:**
```powershell
$env:DB_HOST="localhost"
$env:DB_PORT="5432"
$env:DB_USER="postgres"
$env:DB_PASSWORD="your_password_here"
$env:DB_NAME="devops_db"
$env:JWT_SECRET="your_jwt_secret_here"
```

**Linux/Mac:**
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=your_password_here
export DB_NAME=devops_db
export JWT_SECRET=your_jwt_secret_here
```

**Prompt Manager Tests**

**Option A: Using environment file**

```bash
# Copy and configure test environment
cp services/prompt-manager.service/test.env.example services/prompt-manager.service/test.env
# Edit test.env and update the placeholder values

# Source the file
source services/prompt-manager.service/test.env

# Run tests
cd services/prompt-manager.service
go test -v
```

**Option B: Set environment variables manually**

**Windows PowerShell:**
```powershell
$env:DB_HOST="localhost"
$env:DB_PORT="5433"
$env:DB_USER="postgres"
$env:DB_PASSWORD="your_password_here"
$env:DB_NAME="chats_db"
$env:PUBSUB_EMULATOR_HOST="localhost:8085"
$env:PUBSUB_PROJECT_ID="local-project"
$env:PUBSUB_TOPIC_ID="prompt-requests"
$env:PUBSUB_SUBSCRIPTION_ID="prompt-requests-sub"
$env:AUTH_SERVICE_URL="http://localhost:8001"
$env:GEMMA3_SERVICE_URL="http://localhost:8003"
$env:QWEN3_SERVICE_URL="http://localhost:8004"
```

**Linux/Mac:**
```bash
export DB_HOST=localhost
export DB_PORT=5433
export DB_USER=postgres
export DB_PASSWORD=your_password_here
export DB_NAME=chats_db
export PUBSUB_EMULATOR_HOST=localhost:8085
export PUBSUB_PROJECT_ID=local-project
export PUBSUB_TOPIC_ID=prompt-requests
export PUBSUB_SUBSCRIPTION_ID=prompt-requests-sub
export AUTH_SERVICE_URL=http://localhost:8001
export GEMMA3_SERVICE_URL=http://localhost:8003
export QWEN3_SERVICE_URL=http://localhost:8004
```

Navigate to the prompt-manager.service directory and run tests:

```bash
cd services/prompt-manager.service
go test -v
```

For verbose output with detailed logs:

```bash
go test -v -run TestLoginHandler_Success
```

To run specific tests:

```bash

### Prompt Manager Tests

1. **Public Endpoints:**
   - ✅ Health check endpoint
   - ✅ Available models endpoint

2. **Chat Management Tests:**
   - ✅ Create chat with valid model
   - ✅ Create chat with invalid model
   - ✅ Create chat without authentication (unauthorized)
   - ✅ List chats with pagination
   - ✅ Get specific chat with messages
   - ✅ Get non-existent chat (not found)
   - ✅ Delete chat

3. **Message Tests:**
   - ✅ Send message to chat
   - ✅ Message persistence in database

4. **LLM Integration Tests:**
   - ✅ Process message with mock LLM service
   - ✅ Assistant response generation
   - ✅ Token usage tracking
   - ✅ Message status updates

5. **Authentication & Authorization:**
   - ✅ Auth middleware with valid token
   - ✅ Auth middleware with invalid token
   - ✅ User ID extraction from JWT

6. **Pub/Sub Integration:**
   - ✅ Pub/Sub client connection
   - ✅ Topic creation and access
   - ✅ Message publishing

7. **Database Tests:**
   - ✅ Database connection
   - ✅ Chat CRUD operations
   - ✅ Message CRUD operations
   - ✅ Data persistence and cleanup
go test -v -run TestCreateUserHandler_Success
go test -v -run TestLoginHandler
go test -v -run TestValidateHandler
```

### Step 4: Run API Gateway Tests

Navigate to the API Gateway directory:

```bash
cd services/api-gateway.service
go test -v
```

## Test Coverage

### Auth Service Tests

1. **Authentication Tests:**
   - ✅ User login with valid credentials
   - ✅ User login with invalid credentials
   - ✅ JWT validation (valid token)
   - ✅ JWT validation (invalid token)

2. **Admin Operations Tests:**
   - ✅ Get all users
   - ✅ Create new user
   - ✅ Create user with duplicate username (conflict)
   - ✅ Update user role
   - ✅ Delete user

3. **Database Tests:**
   - ✅ Database connection
   - ✅ Data persistence

### API Gateway Tests

1. **CORS Tests:**
   - ✅ CORS headers on regular requests
   - ✅ CORS preflight (OPTIONS) requests

2. **Routing Tests:**
   - ✅ Reverse proxy to auth service
   - ✅ Admin routes proxying
   - ✅ Invalid route handling

3. **Integration Tests:**
   - ✅ Full flow with CORS
   - ✅ Environment configuration

## Running Tests with Docker Compose

To run tests in a containerized environment:

1. Build and start all services:
```bash
docker-compose up -d
```

2. Execute tests inside the container:
```bash
# Auth service tests
docker-compose exec auth go test -v

# API Gateway tests (if needed)
docker-compose exec api-gateway go test -v
```

## Test Database

Tests use separate databases for different services:
- **Auth Service**: Uses the `devops_db` database (port 5432)
- **Prompt Manager**: Uses the `chats_db` database (port 5433 when testing locally)

Test data is cleaned up after each test using cleanup functions.

### Database Schemas

**Auth Database:**

```sql
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Chats Database:**

```sql
CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL,
    title VARCHAR(255) NOT NULL,
    model VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'completed',
    error_message TEXT,
    tokens_used INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_chats_user_id ON chats(user_id);
```

## Troubleshooting

### Database Connection Issues

If tests fail with database connection errors:

1. Verify the databases are running:
```bash
docker-compose -f docker-compose.yml -f docker-compose.test.yml ps
```

2. Check database logs:
```bash
docker-compose -f docker-compose.yml -f docker-compose.test.yml logs auth-db
docker-compose -f docker-compose.yml -f docker-compose.test.yml logs chats_db
```

3. Verify environment variables are set correctly:
```bash
# Windows
echo $env:DB_HOST
echo $env:DB_PORT

# Linux/Mac
echo $DB_HOST
echo $DB_PORT
```

### Port Conflicts

If ports are already in use:

- Port 5432 (auth-db): Check for existing PostgreSQL instances
- Port 5433 (chats_db): Check for conflicting services
- Port 8085 (Pub/Sub emulator): Check for conflicting applications
- Port 8001 (auth service): Check for running auth service
- Port 8003/8004 (mock LLMs): Check for conflicting services

Solution:
1. Stop conflicting services or change ports in `docker-compose.test.yml`
2. Update environment variables accordingly

### Pub/Sub Emulator Issues

If Pub/Sub tests fail:

1. Verify the emulator is running:
```bash
docker-compose -f docker-compose.yml -f docker-compose.test.yml logs pubsub-emulator
```

2. Check the `PUBSUB_EMULATOR_HOST` environment variable is set:
```bash
# Should be: localhost:8085
echo $PUBSUB_EMULATOR_HOST
```

3. Ensure the emulator is accessible:
```bash
curl http://localhost:8085
# Should return: Ok
```

4. Manually test topic creation:
```bash
# Set the emulator host
export PUBSUB_EMULATOR_HOST=localhost:8085

# Create a test topic (requires gcloud CLI)
gcloud pubsub topics create test-topic --project=local-project
```

### LLM Service Mock Issues

If LLM integration tests fail:

1. Verify mock LLM services are running:
```bash
docker-compose -f docker-compose.yml -f docker-compose.test.yml ps gemma3 qwen3
```

2. Test mock service endpoints:
```bash
curl http://localhost:8003/health
curl http://localhost:8004/health
```

### Database Issues

If database connection tests fail:

1. Check database logs:
```bash
docker-compose -f docker-compose.yml -f docker-compose.test.yml logs auth-db
docker-compose -f docker-compose.yml -f docker-compose.test.yml logs chats_db
```

2. Verify database is accepting connections:
```bash
# Auth database
docker exec -it auth-postgres-db psql -U postgres -d devops_db -c "SELECT 1;"

# Chats database  
docker exec -it chats-db psql -U postgres -d chats_db -c "SELECT 1;"
```

3. Clean up test data:
```bash
# In auth database
docker exec -it auth-postgres-db psql -U postgres -d devops_db -c "DELETE FROM users WHERE username LIKE 'test_%';"

# In chats database
docker exec -it chats-db psql -U postgres -d chats_db -c "DELETE FROM chats WHERE title LIKE 'Test%';"
```

## CI/CD Integration

To integrate these tests into a CI/CD pipeline:

1. **GitHub Actions Example:**

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_DB: devops_db
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432
    

# Prompt Manager
cd services/prompt-manager.service
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Test Architecture

### Prompt Manager Test Design

The prompt-manager integration tests follow these key patterns:

1. **Test Isolation**: Each test uses a `testContext` with its own database connection and mock services
2. **Global State Protection**: A mutex prevents race conditions when tests access global variables
3. **Mock Services**: HTTP test servers simulate auth service and LLM services
4. **Cleanup Handlers**: Deferred cleanup ensures test data is removed even if tests fail
5. **Environment Flexibility**: Tests work with both emulator services and real services

### Mock Services

**Mock Auth Server:**
- Expects JWT-shaped bearer tokens (three base64-encoded segments separated by dots)
- Accepts any properly formatted JWT token except the literal string `"invalid-token"`
- Returns successful `AuthValidateResponse` (userID: 123, username: "testuser", role: "user") for accepted tokens
- Returns authentication error for `"invalid-token"`

**Mock LLM Server:**
- Implements `/completion` endpoint
- Returns predictable responses for testing
- Simulates token usage tracking

## Running Specific Tests

Run individual test functions:

```bash
# Auth service - specific test
cd services/auth.service
go test -v -run TestLoginHandler_Success

# Prompt Manager - specific test
cd services/prompt-manager.service
go test -v -run TestCreateChatHandler_Success
go test -v -run TestProcessMessage_LLMIntegration
go test -v -run TestAuthMiddleware
```

Run tests matching a pattern:

```bash
# All chat-related tests
go test -v -run Chat

# All LLM integration tests
go test -v -run LLM
```

## Coverage Reports

Generate test coverage reports:

```bash
# Auth service
cd services/auth.service
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# API Gateway
cd services/api-gateway.service
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Next Steps

After running integration tests successfully:

1. ✅ Verify all tests pass
2. ✅ Review coverage reports
3. ✅ Add more test cases as needed
4. ✅ Integrate into CI/CD pipeline
5. ✅ Document any edge cases
6. ✅ Set up automated testing on pull requests
7. ✅ Monitor test performance and flakiness
8. ✅ Keep mock services updated with real service APIs

## Additional Resources

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Google Cloud Pub/Sub Emulator](https://cloud.google.com/pubsub/docs/emulator)
- [PostgreSQL Testing Best Practices](https://www.postgresql.org/docs/current/regress.html)
