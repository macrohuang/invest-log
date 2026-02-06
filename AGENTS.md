# Repository Guidelines

## Project Structure & Module Organization
- `go-backend/` houses the Go API server (requires Go 1.22+). Entry point: `go-backend/cmd/server/main.go`. Core domain logic (models, business rules, price fetching) lives in `go-backend/pkg/investlog`, with HTTP handlers in `go-backend/internal/api` and config in `go-backend/internal/config`.
- `static/` contains the SPA assets (HTML/CSS/JS, icons) served by the Go server.
- `ios/` is the Capacitor iOS project; `ios/App/App/public` mirrors `static/` for mobile builds.
- `macos/` contains assets for the native macOS app wrapper (SwiftUI shell that embeds the Go backend).
- `scripts/` has helper utilities (for example, syncing the SPA).
- `logs/` and `output/` are generated at runtime or during builds (gitignored).

## Build, Test, and Development Commands
- Run the API + SPA locally:
  ```bash
  cd go-backend
  go run ./cmd/server --data-dir /path/to/data --port 8000
  ```
  This serves the SPA (auto-detects `../static`). Use `--web-dir` to point at a custom static directory.
- Sync SPA assets into iOS:
  ```bash
  scripts/sync_spa.sh      # macOS/Linux
  scripts/sync_spa.ps1     # Windows (PowerShell)
  ```
- Build a macOS backend binary:
  ```bash
  cd go-backend
  # Apple Silicon (M1/M2/M3)
  GOOS=darwin GOARCH=arm64 go build -o dist/invest-log-backend ./cmd/server
  # Intel Mac
  GOOS=darwin GOARCH=amd64 go build -o dist/invest-log-backend ./cmd/server
  ```

## Coding Style & Naming Conventions
- Go: format with `gofmt` (tabs). Use CamelCase for all identifiers (lowercase first letter for unexported). Go filenames use snake_case (for example, `price_update.go`).
- JS/CSS/HTML: 2-space indentation; keep names in lowerCamelCase; use `const`/`let` (never `var`).

## Testing Guidelines
- Run Go tests with `go test ./...` from the `go-backend/` directory. Tests use standard `*_test.go` files with `TestXxx` functions.
- For UI changes, do a manual pass in the SPA and in the iOS/macOS wrappers: verify responsive layout, check basic functionality, and test on different screen sizes.

## Commit & Pull Request Guidelines
- Commit messages are short, descriptive phrases written in Chinese. Keep them concise and action-oriented (e.g., `修复获取A股场外ETF错误的问题`, `重构设置页面`).
- PRs should include: a brief summary of changes, tests run (or "not run"), screenshots for UI changes, and any data/schema impacts. Keep PRs focused on a single concern.

## Configuration & Data
- The backend uses a SQLite data directory. Override defaults with `INVEST_LOG_DATA_DIR` (sets the data directory) or `INVEST_LOG_DB_PATH` (sets the exact database file path; takes precedence if both are set).
- Logs are written under the data directory in `logs/` with daily rotation (7 days retention).
