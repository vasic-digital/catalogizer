# Contributing to Catalogizer

## Table of Contents
1. [Welcome Contributors](#welcome-contributors)
2. [Getting Started](#getting-started)
3. [Development Environment Setup](#development-environment-setup)
4. [Code Contribution Guidelines](#code-contribution-guidelines)
5. [Testing Requirements](#testing-requirements)
6. [Documentation Guidelines](#documentation-guidelines)
7. [Pull Request Process](#pull-request-process)
8. [Code Review Process](#code-review-process)
9. [Issue Reporting](#issue-reporting)
10. [Security Vulnerabilities](#security-vulnerabilities)
11. [Community Guidelines](#community-guidelines)
12. [Development Best Practices](#development-best-practices)
13. [Release Process](#release-process)

## Welcome Contributors

Thank you for your interest in contributing to Catalogizer v2.4.1! This document provides guidelines and instructions for contributing to the project. Whether you're fixing bugs, adding features, improving documentation, or helping with testing, your contributions are welcome and appreciated.

### Ways to Contribute

- **Code Contributions**: Bug fixes, new features, performance improvements
- **Documentation**: API docs, user guides, tutorials, code comments
- **Testing**: Writing tests, reporting bugs, testing new features
- **Design**: UI/UX improvements, accessibility enhancements
- **Community**: Helping other users, answering questions, mentoring

### Project Values

- **Quality**: We strive for high-quality, maintainable code
- **Security**: Security is a top priority in all contributions
- **Performance**: We optimize for speed and resource efficiency
- **Accessibility**: Features should be accessible to all users
- **Documentation**: Code should be well-documented and self-explanatory

## Getting Started

### Prerequisites

Before contributing, ensure you have:

- Go 1.21 or later
- Git
- A GitHub account
- Basic understanding of media management systems
- Familiarity with REST APIs and databases

### Useful Resources

- [Project Architecture Overview](architecture/ARCHITECTURE.md)
- [API Documentation](api/API_DOCUMENTATION.md)
- [Testing Guide](TESTING_GUIDE.md)
- [Configuration Guide](CONFIGURATION_GUIDE.md)
- [Deployment Guide](DEPLOYMENT_GUIDE.md)
- [Database Schema](architecture/DATABASE_SCHEMA.md)
- [SQL Migrations](architecture/SQL_MIGRATIONS.md)

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: General questions and discussions
- **Email**: security@catalogizer.com (for security issues only)

## Development Environment Setup

### 1. Fork and Clone the Repository

```bash
# Fork the repository on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/catalogizer.git
cd catalogizer/catalog-api

# Add upstream remote
git remote add upstream https://github.com/original-org/catalogizer.git
```

### 2. Install Dependencies

```bash
# Install Go dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### 3. Set Up Development Database

```bash
# Option 1: SQLite (for quick development)
export CATALOGIZER_DB_TYPE=sqlite
export CATALOGIZER_DB_CONNECTION="./dev.db"

# Option 2: PostgreSQL (recommended for full feature testing)
docker run --name catalogizer-postgres \
  -e POSTGRES_DB=catalogizer_dev \
  -e POSTGRES_USER=dev_user \
  -e POSTGRES_PASSWORD=dev_password \
  -p 5432:5432 \
  -d postgres:14

export CATALOGIZER_DB_TYPE=postgresql
export CATALOGIZER_DB_CONNECTION="host=localhost port=5432 user=dev_user password=dev_password dbname=catalogizer_dev sslmode=disable"
```

### 4. Configure Development Environment

```bash
# Create development configuration
mkdir -p config
cat > config/dev.json << EOF
{
  "version": "2.4.1",
  "configuration": {
    "server": {
      "port": 8080,
      "host": "localhost",
      "cors_enabled": true,
      "cors_origins": ["http://localhost:3000"]
    },
    "database": {
      "type": "sqlite",
      "connection_string": "./dev.db",
      "auto_migrate": true
    },
    "storage": {
      "type": "local",
      "path": "./media_dev"
    },
    "logging": {
      "level": "debug",
      "output": "console"
    },
    "features": {
      "analytics_enabled": true,
      "favorites_enabled": true,
      "conversion_enabled": true,
      "sync_enabled": true,
      "stress_testing_enabled": true,
      "error_reporting_enabled": true,
      "log_management_enabled": true
    }
  }
}
EOF

# Set environment variables
export CATALOGIZER_ENV=development
export CATALOGIZER_CONFIG_PATH=config/dev.json
export CATALOGIZER_LOG_LEVEL=debug
```

### 5. Run Development Server

```bash
# Run database migrations
go run main.go --migrate

# Start development server with hot reload
air  # if you have air installed for hot reload

# Or run normally
go run main.go
```

### 6. Verify Setup

```bash
# Test health endpoint
curl http://localhost:8080/health

# Run tests
go test ./...

# Run linting
golangci-lint run
```

## Code Contribution Guidelines

### Project Structure

```
catalog-api/
├── main.go              # Application entry point
├── handlers/            # HTTP request handlers
├── services/            # Business logic services
├── repository/          # Data access layer
├── models/             # Data models and structures
├── middleware/         # HTTP middleware
├── utils/              # Utility functions
├── config/             # Configuration files
├── migrations/         # Database migrations
├── tests/              # Test files
├── docs/               # Documentation
└── scripts/            # Build and deployment scripts
```

### Coding Standards

#### Go Style Guidelines

```go
// Good: Use clear, descriptive function names
func CreateMediaItem(userID int, request *models.CreateMediaItemRequest) (*models.MediaItem, error) {
    // Implementation
}

// Good: Use proper error handling
result, err := service.ProcessMedia(mediaItem)
if err != nil {
    return nil, fmt.Errorf("failed to process media: %w", err)
}

// Good: Use meaningful variable names
var (
    maxRetryAttempts = 3
    retryDelay      = time.Second * 2
    timeout         = time.Minute * 5
)

// Good: Structure with clear separation of concerns
type MediaService struct {
    repo     repository.MediaRepository
    storage  storage.StorageInterface
    logger   *log.Logger
}

func (s *MediaService) CreateMedia(ctx context.Context, req *CreateMediaRequest) (*Media, error) {
    // Validate input
    if err := s.validateCreateRequest(req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // Process business logic
    media, err := s.processMediaCreation(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to create media: %w", err)
    }

    // Store in repository
    if err := s.repo.Create(ctx, media); err != nil {
        return nil, fmt.Errorf("failed to store media: %w", err)
    }

    return media, nil
}
```

#### Naming Conventions

```go
// Variables and functions: camelCase
var mediaItems []MediaItem
func getUserByID(id int) (*User, error)

// Constants: PascalCase or UPPER_CASE for exported
const (
    DefaultTimeout = time.Minute * 5
    MAX_FILE_SIZE = 100 * 1024 * 1024
)

// Types: PascalCase
type MediaItem struct {
    ID        int       `json:"id"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"`
}

// Interfaces: end with -er when possible
type MediaProcessor interface {
    Process(media *MediaItem) error
}

// Package names: lowercase, single word when possible
package handlers
package mediaservice
```

#### Error Handling

```go
// Good: Wrap errors with context
func (s *UserService) CreateUser(req *CreateUserRequest) (*User, error) {
    user, err := s.validateAndCreateUser(req)
    if err != nil {
        return nil, fmt.Errorf("failed to create user %s: %w", req.Username, err)
    }
    return user, nil
}

// Good: Use custom error types for specific cases
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// Good: Handle errors at appropriate levels
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    user, err := h.userService.CreateUser(&req)
    if err != nil {
        var validationErr ValidationError
        if errors.As(err, &validationErr) {
            http.Error(w, validationErr.Error(), http.StatusBadRequest)
            return
        }

        h.logger.Printf("Failed to create user: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

#### Documentation

```go
// Package documentation
// Package mediaservice provides media processing and management functionality.
// It handles media upload, conversion, metadata extraction, and storage operations.
package mediaservice

// Service documentation
// MediaService handles all media-related operations including upload,
// processing, conversion, and metadata management.
type MediaService struct {
    repo     repository.MediaRepository
    storage  storage.StorageInterface
    logger   *log.Logger
}

// Function documentation
// ProcessMedia processes uploaded media files by extracting metadata,
// generating thumbnails, and storing the processed media.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - mediaFile: The uploaded media file to process
//   - options: Processing options such as quality settings
//
// Returns the processed media item or an error if processing fails.
func (s *MediaService) ProcessMedia(ctx context.Context, mediaFile *MediaFile, options *ProcessingOptions) (*MediaItem, error) {
    // Implementation
}
```

### Database Guidelines

#### Migration Files

```sql
-- migrations/001_create_users_table.up.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);

-- migrations/001_create_users_table.down.sql
DROP TABLE IF EXISTS users;
```

#### Repository Pattern

```go
// repository/user_repository.go
type UserRepository interface {
    Create(ctx context.Context, user *models.User) error
    GetByID(ctx context.Context, id int) (*models.User, error)
    GetByUsername(ctx context.Context, username string) (*models.User, error)
    Update(ctx context.Context, user *models.User) error
    Delete(ctx context.Context, id int) error
    List(ctx context.Context, filters *UserFilters) ([]*models.User, error)
}

type userRepository struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
    return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
    query := `
        INSERT INTO users (username, email, password_hash)
        VALUES ($1, $2, $3)
        RETURNING id, created_at, updated_at`

    err := r.db.QueryRowContext(ctx, query, user.Username, user.Email, user.PasswordHash).
        Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

    if err != nil {
        return fmt.Errorf("failed to create user: %w", err)
    }

    return nil
}
```

### API Design Guidelines

#### RESTful Endpoints

```go
// Good: RESTful resource design
GET    /api/users              // List users
POST   /api/users              // Create user
GET    /api/users/{id}         // Get specific user
PUT    /api/users/{id}         // Update user (full update)
PATCH  /api/users/{id}         // Update user (partial update)
DELETE /api/users/{id}         // Delete user

GET    /api/users/{id}/media   // Get user's media items
POST   /api/users/{id}/media   // Create media for user

// Good: Consistent response format
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *APIError   `json:"error,omitempty"`
    Meta    *Meta       `json:"meta,omitempty"`
}

type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

type Meta struct {
    Page       int `json:"page,omitempty"`
    PerPage    int `json:"per_page,omitempty"`
    Total      int `json:"total,omitempty"`
    TotalPages int `json:"total_pages,omitempty"`
}
```

#### Request Validation

```go
// Good: Comprehensive request validation
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=50,alphanum"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

func (r *CreateUserRequest) Validate() error {
    validate := validator.New()

    if err := validate.Struct(r); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // Custom validation logic
    if strings.Contains(strings.ToLower(r.Username), "admin") {
        return errors.New("username cannot contain 'admin'")
    }

    return nil
}
```

### Security Guidelines

#### Authentication and Authorization

```go
// Good: Secure JWT handling
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
    claims := &Claims{}

    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        // Validate signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return s.jwtSecret, nil
    })

    if err != nil {
        return nil, fmt.Errorf("failed to parse token: %w", err)
    }

    if !token.Valid {
        return nil, errors.New("invalid token")
    }

    // Check expiration
    if claims.ExpiresAt < time.Now().Unix() {
        return nil, errors.New("token expired")
    }

    return claims, nil
}

// Good: Input sanitization
func sanitizeInput(input string) string {
    // Remove potentially dangerous characters
    input = strings.TrimSpace(input)
    input = html.EscapeString(input)

    // Remove null bytes
    input = strings.ReplaceAll(input, "\x00", "")

    return input
}

// Good: Password hashing
func (s *AuthService) HashPassword(password string) (string, error) {
    // Use bcrypt with appropriate cost
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", fmt.Errorf("failed to hash password: %w", err)
    }

    return string(hash), nil
}
```

#### SQL Injection Prevention

```go
// Good: Use parameterized queries
func (r *userRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
    query := `SELECT id, username, email, created_at FROM users WHERE username = $1`

    var user models.User
    err := r.db.QueryRowContext(ctx, query, username).Scan(
        &user.ID,
        &user.Username,
        &user.Email,
        &user.CreatedAt,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrUserNotFound
        }
        return nil, fmt.Errorf("failed to get user: %w", err)
    }

    return &user, nil
}

// Bad: String concatenation (vulnerable to SQL injection)
// NEVER DO THIS
query := fmt.Sprintf("SELECT * FROM users WHERE username = '%s'", username)
```

## Testing Requirements

### Unit Tests

```go
// tests/services/user_service_test.go
func TestUserService_CreateUser(t *testing.T) {
    tests := []struct {
        name    string
        request *CreateUserRequest
        wantErr bool
        errType error
    }{
        {
            name: "valid user creation",
            request: &CreateUserRequest{
                Username: "testuser",
                Email:    "test@example.com",
                Password: "securepassword123",
            },
            wantErr: false,
        },
        {
            name: "invalid email",
            request: &CreateUserRequest{
                Username: "testuser",
                Email:    "invalid-email",
                Password: "securepassword123",
            },
            wantErr: true,
            errType: ValidationError{},
        },
        {
            name: "duplicate username",
            request: &CreateUserRequest{
                Username: "existinguser",
                Email:    "new@example.com",
                Password: "securepassword123",
            },
            wantErr: true,
            errType: DuplicateUserError{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup
            mockRepo := &mockUserRepository{}
            service := NewUserService(mockRepo)

            // Execute
            user, err := service.CreateUser(tt.request)

            // Assert
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errType != nil {
                    assert.IsType(t, tt.errType, err)
                }
                assert.Nil(t, user)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, user)
                assert.Equal(t, tt.request.Username, user.Username)
                assert.Equal(t, tt.request.Email, user.Email)
            }
        })
    }
}

// Mock repository for testing
type mockUserRepository struct {
    users map[int]*models.User
    nextID int
}

func (m *mockUserRepository) Create(ctx context.Context, user *models.User) error {
    // Check for duplicate username
    for _, existingUser := range m.users {
        if existingUser.Username == user.Username {
            return DuplicateUserError{Field: "username"}
        }
    }

    m.nextID++
    user.ID = m.nextID
    user.CreatedAt = time.Now()
    user.UpdatedAt = time.Now()

    if m.users == nil {
        m.users = make(map[int]*models.User)
    }
    m.users[user.ID] = user

    return nil
}
```

### Integration Tests

```go
// tests/integration/user_api_test.go
func TestUserAPI_Integration(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer db.Close()

    // Setup test server
    server := setupTestServer(db)
    defer server.Close()

    // Test user creation
    t.Run("create user", func(t *testing.T) {
        payload := CreateUserRequest{
            Username: "testuser",
            Email:    "test@example.com",
            Password: "securepassword123",
        }

        body, _ := json.Marshal(payload)
        resp, err := http.Post(server.URL+"/api/users", "application/json", bytes.NewBuffer(body))

        assert.NoError(t, err)
        assert.Equal(t, http.StatusCreated, resp.StatusCode)

        var response APIResponse
        err = json.NewDecoder(resp.Body).Decode(&response)
        assert.NoError(t, err)
        assert.True(t, response.Success)

        user := response.Data.(map[string]interface{})
        assert.Equal(t, payload.Username, user["username"])
        assert.Equal(t, payload.Email, user["email"])
    })
}

func setupTestDB(t *testing.T) *sql.DB {
    // Create in-memory SQLite database for testing
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatalf("Failed to create test database: %v", err)
    }

    // Run migrations
    if err := runMigrations(db); err != nil {
        t.Fatalf("Failed to run migrations: %v", err)
    }

    return db
}
```

### Test Coverage Requirements

- **Minimum Coverage**: 80% for all packages
- **Critical Paths**: 95% coverage for authentication, authorization, and data validation
- **Integration Tests**: All API endpoints must have integration tests
- **Performance Tests**: Critical paths must have benchmark tests

```bash
# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Check coverage percentage
go tool cover -func=coverage.out | grep total
```

## Documentation Guidelines

### Code Documentation

```go
// Package documentation
// Package analytics provides comprehensive analytics and event tracking
// functionality for the Catalogizer application. It includes real-time
// event collection, aggregation, and reporting capabilities.
//
// The package is organized into the following components:
//   - EventCollector: Collects and validates incoming events
//   - Aggregator: Processes and aggregates event data
//   - Reporter: Generates analytics reports
//
// Example usage:
//   collector := analytics.NewEventCollector(config)
//   event := &analytics.Event{
//       Type: "media_upload",
//       UserID: 123,
//       Properties: map[string]interface{}{
//           "file_size": 1024000,
//           "file_type": "image/jpeg",
//       },
//   }
//   err := collector.Track(event)
package analytics

// Type documentation with examples
// Event represents a single analytics event with associated metadata.
// Events are used to track user interactions, system operations, and
// business metrics throughout the application.
//
// Example:
//   event := &Event{
//       Type: "user_login",
//       UserID: 456,
//       Timestamp: time.Now(),
//       Properties: map[string]interface{}{
//           "login_method": "email",
//           "device_type": "mobile",
//       },
//   }
type Event struct {
    ID         string                 `json:"id"`
    Type       string                 `json:"type"`
    UserID     int                    `json:"user_id"`
    Timestamp  time.Time              `json:"timestamp"`
    Properties map[string]interface{} `json:"properties"`
}

// Method documentation with parameters and return values
// Track records an analytics event for processing and storage.
// The event is validated, enriched with additional metadata,
// and queued for batch processing.
//
// Parameters:
//   - event: The event to track (must not be nil)
//
// Returns:
//   - error: nil on success, or an error describing the failure
//
// The method will return an error if:
//   - The event is nil
//   - The event type is empty or invalid
//   - The event fails validation
//   - The storage queue is full
//
// Example:
//   err := collector.Track(&Event{
//       Type: "page_view",
//       UserID: 123,
//       Properties: map[string]interface{}{
//           "page": "/dashboard",
//           "referrer": "/login",
//       },
//   })
//   if err != nil {
//       log.Printf("Failed to track event: %v", err)
//   }
func (c *EventCollector) Track(event *Event) error {
    // Implementation
}
```

### API Documentation

Use Swagger/OpenAPI 3.0 annotations:

```go
// @title Catalogizer API
// @version 2.4.1
// @description Comprehensive media management and cataloging system
// @contact.name API Support
// @contact.email api-support@catalogizer.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /api

// @Summary Create a new user
// @Description Create a new user account with the provided information
// @Tags users
// @Accept json
// @Produce json
// @Param user body CreateUserRequest true "User creation request"
// @Success 201 {object} APIResponse{data=User} "User created successfully"
// @Failure 400 {object} APIResponse{error=APIError} "Invalid request data"
// @Failure 409 {object} APIResponse{error=APIError} "User already exists"
// @Failure 500 {object} APIResponse{error=APIError} "Internal server error"
// @Router /users [post]
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// Generate swagger docs
// swag init -g main.go --output docs/swagger
```

### README Updates

When adding new features, update relevant README sections:

```markdown
## New Feature: Advanced Analytics

### Overview
The advanced analytics system provides real-time insights into user behavior,
system performance, and content usage patterns.

### Features
- Real-time event tracking
- Customizable dashboards
- Automated report generation
- Data export capabilities

### Usage

#### Basic Event Tracking
```go
event := &analytics.Event{
    Type: "media_upload",
    UserID: userID,
    Properties: map[string]interface{}{
        "file_size": fileSize,
        "file_type": mimeType,
    },
}
err := analyticsService.Track(event)
```

#### Dashboard Access
Navigate to `/analytics` in the web interface to view real-time dashboards
and generate custom reports.

### Configuration
```json
{
  "analytics": {
    "enabled": true,
    "batch_size": 100,
    "flush_interval": "30s",
    "retention_days": 90
  }
}
```

## Pull Request Process

### 1. Before Creating a Pull Request

```bash
# Update your fork
git fetch upstream
git checkout main
git merge upstream/main
git push origin main

# Create feature branch
git checkout -b feature/your-feature-name

# Make your changes and commit
git add .
git commit -m "feat: add advanced analytics dashboard"

# Run tests and linting
go test ./...
golangci-lint run

# Update documentation if needed
# Push to your fork
git push origin feature/your-feature-name
```

### 2. Pull Request Template

When creating a pull request, use this template:

```markdown
## Description
Brief description of the changes and why they were made.

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Code refactoring

## Related Issues
Closes #123
Related to #456

## Changes Made
- Added new analytics service with real-time event tracking
- Implemented dashboard API endpoints
- Added comprehensive test coverage
- Updated API documentation

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed
- [ ] Performance testing completed (if applicable)

## Documentation
- [ ] Code comments updated
- [ ] API documentation updated
- [ ] README updated
- [ ] Migration guide provided (if breaking change)

## Screenshots (if applicable)
[Add screenshots of UI changes]

## Performance Impact
Describe any performance implications of the changes.

## Security Considerations
Describe any security implications of the changes.

## Breaking Changes
List any breaking changes and migration steps required.

## Checklist
- [ ] My code follows the project's style guidelines
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] My changes generate no new warnings
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
- [ ] Any dependent changes have been merged and published
```

### 3. Pull Request Guidelines

- **Title**: Use conventional commit format (feat:, fix:, docs:, etc.)
- **Description**: Provide clear description of changes and rationale
- **Size**: Keep PRs focused and reasonably sized (< 500 lines of code when possible)
- **Tests**: Include appropriate tests for new functionality
- **Documentation**: Update documentation for user-facing changes
- **Breaking Changes**: Clearly mark and document breaking changes

## Code Review Process

### Review Criteria

#### Code Quality
- [ ] Code follows project style guidelines
- [ ] Code is readable and well-documented
- [ ] No code duplication
- [ ] Appropriate error handling
- [ ] Secure coding practices followed

#### Functionality
- [ ] Feature works as described
- [ ] Edge cases are handled
- [ ] No regression issues
- [ ] Performance is acceptable

#### Testing
- [ ] Adequate test coverage
- [ ] Tests are meaningful and comprehensive
- [ ] Tests pass consistently
- [ ] Integration tests included for API changes

#### Documentation
- [ ] Code is properly documented
- [ ] API documentation updated
- [ ] User documentation updated
- [ ] Breaking changes documented

### Review Process

1. **Automated Checks**: CI/CD pipeline runs tests and linting
2. **Peer Review**: At least one team member reviews the code
3. **Maintainer Review**: Project maintainer provides final approval
4. **Merge**: Changes are merged after all approvals

### Addressing Review Comments

```bash
# Make requested changes
git add .
git commit -m "address review comments: improve error handling"

# Force push to update PR (only if necessary)
git push origin feature/your-feature-name

# Or amend the last commit if it's a small fix
git add .
git commit --amend --no-edit
git push --force-with-lease origin feature/your-feature-name
```

## Issue Reporting

### Bug Reports

Use this template for bug reports:

```markdown
## Bug Description
A clear and concise description of what the bug is.

## Steps to Reproduce
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error

## Expected Behavior
A clear and concise description of what you expected to happen.

## Actual Behavior
A clear and concise description of what actually happened.

## Environment
- OS: [e.g. Ubuntu 20.04]
- Go Version: [e.g. 1.21.5]
- Catalogizer Version: [e.g. 2.4.1]
- Database: [e.g. PostgreSQL 14]

## Logs
```
[Include relevant log entries]
```

## Screenshots
If applicable, add screenshots to help explain your problem.

## Additional Context
Add any other context about the problem here.
```

### Feature Requests

Use this template for feature requests:

```markdown
## Feature Description
A clear and concise description of the feature you'd like to see.

## Problem Statement
Describe the problem this feature would solve.

## Proposed Solution
Describe the solution you'd like to see implemented.

## Alternatives Considered
Describe any alternative solutions or features you've considered.

## Use Cases
Provide specific use cases where this feature would be beneficial.

## Implementation Considerations
Any technical considerations or challenges you foresee.

## Additional Context
Add any other context or screenshots about the feature request here.
```

## Security Vulnerabilities

### Reporting Security Issues

**DO NOT** create public GitHub issues for security vulnerabilities.

Instead, email security details to: **security@catalogizer.com**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if known)

### Security Review Process

1. **Initial Response**: Within 24 hours
2. **Severity Assessment**: Within 72 hours
3. **Fix Development**: Based on severity
4. **Coordinated Disclosure**: After fix is ready

### Security Best Practices

```go
// Input validation
func validateUserInput(input string) error {
    if len(input) > MAX_INPUT_LENGTH {
        return errors.New("input too long")
    }

    if containsUnsafeCharacters(input) {
        return errors.New("input contains unsafe characters")
    }

    return nil
}

// SQL injection prevention
func (r *repository) getUserByID(id int) (*User, error) {
    query := "SELECT * FROM users WHERE id = ?"
    // Use parameterized queries, never string concatenation
    row := r.db.QueryRow(query, id)
    // ... rest of implementation
}

// Authentication
func (h *handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        token := extractToken(r)
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        claims, err := validateToken(token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        // Add user context
        ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    }
}
```

## Community Guidelines

### Code of Conduct

We are committed to providing a welcoming and inclusive environment for all contributors.

#### Expected Behavior
- Be respectful and inclusive
- Welcome newcomers and help them learn
- Give constructive feedback
- Focus on the issue, not the person
- Acknowledge different viewpoints

#### Unacceptable Behavior
- Harassment or discriminatory language
- Personal attacks or insults
- Trolling or inflammatory comments
- Publishing private information
- Any conduct that would be inappropriate in a professional setting

### Communication Guidelines

#### GitHub Issues and Pull Requests
- Use clear, descriptive titles
- Provide context and background
- Be patient with responses
- Stay on topic
- Use appropriate labels and milestones

#### Code Reviews
- Be constructive and specific
- Explain the "why" behind suggestions
- Acknowledge good work
- Focus on the code, not the author
- Suggest improvements, don't just point out problems

## Development Best Practices

### Git Workflow

```bash
# Keep your fork updated
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branches from main
git checkout -b feature/feature-name

# Make small, focused commits
git add specific-file.go
git commit -m "feat: add user authentication middleware"

# Write meaningful commit messages
git commit -m "fix: resolve race condition in media processor

- Add mutex to protect concurrent access to processing queue
- Add test cases for concurrent processing scenarios
- Update error handling to be more specific

Fixes #123"

# Rebase before pushing (to keep clean history)
git rebase main
git push origin feature/feature-name
```

### Commit Message Format

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

Examples:
```
feat(auth): add JWT token refresh mechanism

fix(api): resolve null pointer exception in user handler

docs: update API documentation for new endpoints

test(media): add integration tests for file upload

refactor(db): optimize query performance for user lookup
```

### Performance Considerations

```go
// Good: Use context for cancellation and timeouts
func (s *MediaService) ProcessMedia(ctx context.Context, file *MediaFile) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Processing logic with context checking
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Continue processing
    }
}

// Good: Use connection pooling
func NewDatabaseConnection(config *DBConfig) (*sql.DB, error) {
    db, err := sql.Open("postgres", config.ConnectionString)
    if err != nil {
        return nil, err
    }

    db.SetMaxOpenConns(config.MaxOpenConns)
    db.SetMaxIdleConns(config.MaxIdleConns)
    db.SetConnMaxLifetime(config.ConnMaxLifetime)

    return db, nil
}

// Good: Use buffered channels for async processing
func (s *EventProcessor) StartProcessing() {
    eventChan := make(chan *Event, 1000) // Buffered channel

    for i := 0; i < s.workerCount; i++ {
        go s.worker(eventChan)
    }

    // Event distribution logic
}
```

### Logging Best Practices

```go
// Good: Structured logging with context
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
    logger := s.logger.With("operation", "create_user", "username", req.Username)

    logger.Info("Creating new user")

    user, err := s.processUserCreation(req)
    if err != nil {
        logger.Error("Failed to create user", "error", err)
        return nil, err
    }

    logger.Info("User created successfully", "user_id", user.ID)
    return user, nil
}

// Good: Log levels
logger.Debug("Detailed debug information")  // Development only
logger.Info("General information")           // Normal operations
logger.Warn("Warning conditions")           // Potential issues
logger.Error("Error conditions")            // Error handling
logger.Fatal("Fatal errors")                // System shutdown
```

## Release Process

### Version Numbering

We follow [Semantic Versioning](https://semver.org/) (SemVer):

- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible functionality additions
- **PATCH**: Backward-compatible bug fixes

Examples:
- `2.4.0` - Major release with breaking changes
- `3.1.0` - Minor release with new features
- `3.1.1` - Patch release with bug fixes

### Release Workflow

1. **Feature Freeze**: Stop adding new features
2. **Testing**: Comprehensive testing of release candidate
3. **Documentation**: Update all documentation
4. **Release Notes**: Prepare detailed release notes
5. **Tagging**: Create git tag with version number
6. **Build**: Build and test release artifacts
7. **Distribution**: Publish release

### Release Checklist

#### Pre-Release
- [ ] All planned features completed
- [ ] All tests passing
- [ ] Documentation updated
- [ ] Security review completed
- [ ] Performance testing completed
- [ ] Breaking changes documented
- [ ] Migration guides prepared

#### Release
- [ ] Version bumped in all files
- [ ] Git tag created
- [ ] Release notes published
- [ ] Artifacts built and tested
- [ ] Release published
- [ ] Announcement made

#### Post-Release
- [ ] Monitor for issues
- [ ] Update documentation site
- [ ] Plan next release
- [ ] Backport critical fixes if needed

---

Thank you for contributing to Catalogizer v2.4.1! Your contributions help make this project better for everyone. If you have questions about contributing, please don't hesitate to ask in our GitHub Discussions or reach out to the maintainers.

For more information, see:
- [Architecture Overview](architecture/ARCHITECTURE.md)
- [API Documentation](api/API_DOCUMENTATION.md)
- [Testing Guide](TESTING_GUIDE.md)
- [Deployment Guide](DEPLOYMENT_GUIDE.md)
- [Changelog](CHANGELOG.md)