# Repository Guidelines

## Project Structure & Module Organization
- `go-backend/` houses the Go API server. Entry point: `go-backend/cmd/server/main.go`. Core domain logic lives in `go-backend/pkg/investlog`, with HTTP handlers in `go-backend/internal/api` and config in `go-backend/internal/config`.
- `static/` contains the SPA assets (HTML/CSS/JS, icons) served by the Go server.
- `ios/` is the Capacitor iOS project; `ios/App/App/public` mirrors `static/` for mobile builds.
- `macos/` contains the macOS wrapper assets and build helpers.
- `scripts/` has helper utilities (for example, syncing the SPA). `logs/` and `output/` are generated at runtime or during builds.

## Build, Test, and Development Commands
- Run the API + SPA locally:
  ```bash
  cd go-backend
  go run ./cmd/server --data-dir /path/to/data --port 8000
  ```
  This serves the SPA (auto-detects `../static`). Use `--web-dir` to point at a custom static directory.
- Sync SPA assets into iOS:
  ```bash
  scripts/sync_spa.sh
  ```
- Build a macOS backend binary:
  ```bash
  cd go-backend
  GOOS=darwin GOARCH=arm64 go build -o dist/invest-log-backend ./cmd/server
  ```

## Coding Style & Naming Conventions
- Go: format with `gofmt` (tabs). Use CamelCase for exported identifiers and lowerCamelCase for unexported. Go filenames use snake_case (for example, `price_update.go`).
- JS/CSS/HTML: 2-space indentation; keep names in lowerCamelCase; prefer `const`/`let`.

## Testing Guidelines
- No automated test suite is present. If adding tests, use Go `*_test.go` files with `TestXxx` functions and run `go test ./...`.
- For UI changes, do a manual pass in the SPA and in the iOS/macOS wrappers when relevant.

## Commit & Pull Request Guidelines
- Commit messages are short, descriptive phrases and are currently written in Chinese. Keep them concise and action-oriented.
- PRs should include a brief summary, tests run (or "not run"), screenshots for UI changes, and any data/schema impacts.

## Configuration & Data
- The backend uses a SQLite data directory. Override defaults with `INVEST_LOG_DATA_DIR` or `INVEST_LOG_DB_PATH`.
- Logs are written under the data directory; see `go-backend/README.md` for details.
