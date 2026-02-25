# Repository Guidelines

## Project Structure & Module Organization

- `go-backend/` houses the Go API server (requires Go 1.22+). Entry point: `go-backend/cmd/server/main.go`. Core domain logic (models, business rules, price fetching) lives in `go-backend/pkg/investlog`, with HTTP handlers in `go-backend/internal/api` and config in `go-backend/internal/config`.
- `static/` contains the SPA assets (HTML/CSS/JS, icons) served by the Go server.
- `ios/` is the Capacitor iOS project; `ios/App/App/public` mirrors `static/` for mobile builds.
- `macos/` contains assets for the native macOS app wrapper (SwiftUI shell that embeds the Go backend).
- `scripts/` has helper utilities (for example, syncing the SPA).
- `logs/` and `output/` are generated at runtime or during builds (gitignored).

## Build, Test, and Development Commands

### Running the Application
```bash
cd go-backend
go run ./cmd/server --data-dir /path/to/data --port 8000
```
This serves the SPA (auto-detects `../static`). Use `--web-dir` to point at a custom static directory.

### Testing Commands
```bash
# Run all tests
cd go-backend
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a single test function
go test -v -run TestAddTransaction_Basic ./pkg/investlog/...

# Run tests for a specific package
go test ./pkg/investlog/...
go test ./internal/api/...

# Run tests with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Linting and Formatting
```bash
# Format Go code (uses tabs)
cd go-backend
gofmt -w .

# Check formatting without modifying files
gofmt -l .

# Vet code for issues
go vet ./...
```

### Building
```bash
cd go-backend

# Apple Silicon (M1/M2/M3)
GOOS=darwin GOARCH=arm64 go build -o dist/invest-log-backend ./cmd/server

# Intel Mac
GOOS=darwin GOARCH=amd64 go build -o dist/invest-log-backend ./cmd/server

# Cross-compile for other platforms
GOOS=linux GOARCH=amd64 go build -o dist/invest-log-backend-linux ./cmd/server
GOOS=windows GOARCH=amd64 go build -o dist/invest-log-backend.exe ./cmd/server
```

### Sync SPA to iOS
```bash
scripts/sync_spa.sh      # macOS/Linux
scripts/sync_spa.ps1     # Windows (PowerShell)
```

## Coding Style & Naming Conventions

### Go

#### Formatting
- Use `gofmt` (tabs for indentation)
- Keep lines under 100 characters when possible
- Group imports: stdlib first, then external packages, then internal packages
- Separate import groups with a blank line

#### Naming
- **CamelCase** for all identifiers (lowercase first letter for unexported)
- Go filenames use `snake_case` (e.g., `price_update.go`, `handlers_test.go`)
- Test files use `*_test.go` suffix
- Exported functions/methods should have documentation comments

#### Types and Interfaces
- Use `Core` struct for domain logic access
- Define request/response structs in the same file as the handler
- Use pointer receivers for methods that modify state
- Use value receivers for read-only methods

#### Error Handling
- Return errors directly, don't wrap unless adding context
- Use `fmt.Errorf` with context for user-facing errors
- Check errors immediately after function calls
- Use early returns to reduce nesting

#### Imports Example
```go
import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"modernc.org/sqlite"

	"investlog/internal/config"
	"investlog/pkg/investlog"
)
```

### JavaScript/CSS/HTML

#### Formatting
- **2-space indentation**
- Use `const` and `let` (never `var`)
- Prefer arrow functions for callbacks
- Use template literals for string interpolation

#### Naming
- `lowerCamelCase` for variables, functions, and methods
- `UPPER_SNAKE_CASE` for constants
- Use descriptive names; avoid single-letter variables except in loops

#### Structure
- Keep functions small and focused
- Use early returns to reduce nesting
- Organize code into logical sections with comments

## Testing Guidelines

### Go Test Patterns
- Tests use standard `*_test.go` files with `TestXxx` functions
- Use table-driven tests for multiple test cases
- Use `t.Helper()` in helper functions
- Create temporary databases for integration tests
- Use `setupTestDB()` helper for database tests

### Test Helpers
```go
// Common test helpers available:
setupTestDB(t)                    // Creates temp DB
testAccount(t, core, id, name)    // Creates test account
testBuyTransaction(...)           // Creates BUY transaction
testSellTransaction(...)          // Creates SELL transaction
assertNoError(t, err, msg)        // Asserts no error
assertError(t, err, msg)          // Asserts error exists
assertFloatEquals(t, got, want)   // Compares floats with epsilon
assertContains(t, s, substr)      // Asserts substring exists
```

### UI Testing
For UI changes, do a manual pass in the SPA and in the iOS/macOS wrappers:
- Verify responsive layout on different screen sizes
- Check basic functionality (navigation, forms, buttons)
- Test on both mobile and desktop views

## Commit & Pull Request Guidelines

### Commit Messages
- Write short, descriptive phrases in **Chinese**
- Keep them concise and action-oriented
- Examples:
  - `修复获取A股场外ETF错误的问题`
  - `重构设置页面`
  - `添加持仓分析功能`
  - `优化数据库查询性能`

### Pull Requests
- Include a brief summary of changes
- Mention tests run (or "not run")
- Include screenshots for UI changes
- Document any data/schema impacts
- Keep PRs focused on a single concern

## Configuration & Data

### Environment Variables
- `INVEST_LOG_DATA_DIR` - Sets the data directory
- `INVEST_LOG_DB_PATH` - Sets the exact database file path (takes precedence)
- `INVEST_LOG_PARENT_WATCH` - Set to "1" to enable parent process watching

### Logging

Logs are written under the data directory in `logs/` with daily rotation (7 days retention).
- Logs are written to both **stdout** and a **daily log file** (`logs/app-YYYYMMDD.log`)
- File prefix is `app` (defined in `go-backend/internal/logging/logging.go` as `defaultPrefix`)
- **Never** change the log file prefix from `app` without explicit instruction
- Use structured logging with `slog` package — all application log output **must** go through `slog.Logger`
- **Never** use `fmt.Println`, `log.Println`, or other raw print functions for application logging, as they bypass the log file
- Log levels can be overridden via `INVEST_LOG_LOG_LEVEL` env var (debug, info, warn, error)
- Log format can be switched to JSON via `INVEST_LOG_LOG_FORMAT=json`

## Common Patterns

### Database Access
```go
// Query pattern
rows, err := c.db.Query("SELECT ...")
if err != nil {
    return nil, err
}
defer rows.Close()

// Single row
var result Type
err := c.db.QueryRow("SELECT ...").Scan(&result)
```

### HTTP Handlers
```go
func (h *handler) handlerName(w http.ResponseWriter, r *http.Request) {
    result, err := h.core.MethodName()
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    writeJSON(w, http.StatusOK, result)
}
```

### Error Handling
```go
// Return errors directly
if err != nil {
    return nil, err
}

// Add context for user errors
if !isValidType(req.Type) {
    return nil, fmt.Errorf("invalid type: %s", req.Type)
}

// Early return pattern
if req.ID == "" {
    return nil, errors.New("id required")
}
```
