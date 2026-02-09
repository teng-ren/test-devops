# Acceptance Tests

## Overview
Acceptance tests verify complete user workflows and business requirements for the AI Model Tester platform. These tests ensure all services work together to deliver expected user experiences.

## Test Coverage

### AC1: User Login and Authentication Flow
**Business Value:** Users can authenticate and access the platform  
**Acceptance Criteria:**
- ✅ Existing user can login with valid credentials
- ✅ User receives JWT token upon successful login
- ✅ Token can be used for authenticated requests

### AC2: Admin User Management Flow
**Business Value:** Admins can manage platform users  
**Acceptance Criteria:**
- ✅ Admin can login with admin credentials
- ✅ Admin can view all users
- ✅ Admin can create new users
- ✅ Admin can update user roles

### AC3: Prompt/Chat Creation Flow
**Business Value:** Users can interact with AI models  
**Acceptance Criteria:**
- ✅ Authenticated user can create a new chat/prompt
- ✅ User can retrieve their chat history
- ✅ User can view available models

### AC4: Unauthorized Access Prevention
**Business Value:** Platform security and access control  
**Acceptance Criteria:**
- ✅ Unauthenticated users cannot access protected resources
- ✅ Invalid tokens are rejected
- ✅ Regular users cannot access admin endpoints

### AC5: End-to-End User Journey
**Business Value:** Complete user experience validation  
**Acceptance Criteria:**
- ✅ Complete workflow from login to prompt submission
- ✅ All integrated services communicate properly
- ✅ Seamless user experience across the platform

## Running Tests Locally

### Prerequisites
- All services running locally (auth, api-gateway, prompt-manager)
- API Gateway accessible at `http://localhost:8000`
- Go 1.21 or higher installed

### Execute Tests
```bash
# From the services directory
cd services
go test -v ./acceptance_test.go

# Run specific test
go test -v -run TestUserLoginAndAuthenticationFlow ./acceptance_test.go

# Run with timeout
go test -v ./acceptance_test.go -timeout 5m
```

## CI/CD Integration
Acceptance tests run automatically in the **Dev Branch CI** pipeline after:
- Services are deployed to Kubernetes
- Port forwarding is established
- Integration tests pass

**Pipeline Position:** Between Integration Tests and DAST Security Scan

### CI Test Scope
Due to CI resource constraints, only **auth/admin services** are deployed in the CI environment.

**Tests run in CI:**
- ✅ TestUserLoginAndAuthenticationFlow
- ✅ TestAdminUserManagementFlow
- ✅ TestUnauthorizedAccessPrevention

**Tests skipped in CI** (require prompt-manager deployment):
- ⏭️ TestPromptCreationFlow
- ⏭️ TestEndToEndUserJourney

**Full test suite** runs when you test locally with all services deployed via docker-compose.

## Test Maintenance

### Adding New Tests
1. Identify the business workflow to test
2. Define clear acceptance criteria
3. Create test function following pattern: `TestWorkflowName`
4. Document in this README

### Environment Variables
Tests use these environment variables (with defaults):
- `DEFAULT_ADMIN_USERNAME` (default: "admin")
- `DEFAULT_ADMIN_PASSWORD` (default: "adminpass")
- `DEFAULT_TESTUSER_USERNAME` (default: "testuser")
- `DEFAULT_TESTUSER_PASSWORD` (default: "testpass")

## Difference from Other Test Types

| Test Type | Scope | Purpose |
|-----------|-------|---------|
| **Unit Tests** | Individual functions | Code correctness |
| **Integration Tests** | Service-to-service | Component interaction |
| **Acceptance Tests** | Complete workflows | Business requirements |
| **DAST** | Running application | Security vulnerabilities |
| **Performance Tests** | System under load | Scalability & speed |

## Success Criteria
All acceptance tests must pass before:
- Deploying to staging environment
- Merging to main branch
- Production releases

## Troubleshooting

### Test Failures
1. Check if all services are running: `kubectl get pods -n dev`
2. Verify port forwarding: `curl http://localhost:8000`
3. Check service logs: `kubectl logs -n dev -l app=api-gateway`
4. Verify environment variables are set

### Common Issues
- **Connection refused:** Services not running or port-forward failed
- **401 Unauthorized:** Token authentication issues
- **500 Server Error:** Backend service errors, check logs
- **Timeout:** Services too slow to respond, check resource limits
