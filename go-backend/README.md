# Invest Log Go Backend

This folder contains the Go refactor of the Invest Log backend. It keeps the same SQLite schema and core business rules, while exposing a JSON API and a SPA-friendly static server for desktop/mobile clients.

## Quick Start

```bash
cd go-backend
# download deps
# go mod tidy

# run server + SPA (uses ../static if available)
go run ./cmd/server --data-dir /path/to/data --port 8000
```

Optional flags:
- `--web-dir`: path to SPA static files (defaults to `static` or `../static` if found)

Environment variables:
- `INVEST_LOG_DATA_DIR`: override data directory
- `INVEST_LOG_DB_PATH`: override DB file path
- `INVEST_LOG_LOG_LEVEL`: override log level (`debug`/`info`/`warn`/`error`)
- `INVEST_LOG_LOG_FORMAT`: log output format (`text` or `json`)

Logs are written to `logs/` under the data directory with daily rotation (7 days).
API requests are logged with request ID, status code, latency, client IP, and user agent.

## API (JSON)

Core endpoints:
- `GET /api/health`
- `GET /api/holdings`
- `GET /api/holdings-by-currency`
- `GET /api/holdings-by-symbol`
- `GET /api/transactions`
- `POST /api/transactions`
- `DELETE /api/transactions/{id}`
- `GET /api/portfolio-history`

Operational endpoints:
- `POST /api/prices/update`
- `POST /api/prices/manual`
- `POST /api/prices/update-all`
- `POST /api/ai/holdings-analysis`
- `GET /api/accounts`
- `POST /api/accounts`
- `DELETE /api/accounts/{id}`
- `GET /api/asset-types`
- `POST /api/asset-types`
- `DELETE /api/asset-types/{code}`
- `GET /api/allocation-settings`
- `PUT /api/allocation-settings`
- `DELETE /api/allocation-settings`
- `GET /api/symbols`
- `PUT /api/symbols/{symbol}`
- `POST /api/symbols/{symbol}/asset-type`
- `POST /api/symbols/{symbol}/auto-update`
- `GET /api/operation-logs`

## SPA Frontend

The SPA lives in the repo `static/` directory and calls the Go API. When running the server from `go-backend`, it auto-detects `../static` and serves it at `/`.

If you open `static/index.html` directly (file or Capacitor), set the API base in Settings or pass `?api=http://127.0.0.1:8000`.

The Holdings page includes an `AI Analyze` action that calls
`/api/ai/holdings-analysis`. It accepts OpenAI-compatible `base_url`, `model`,
`api_key`, and optional `strategy_prompt`, and returns structured analysis plus
symbol-level suggestions.

## macOS build

```bash
cd go-backend
mkdir -p dist
GOOS=darwin GOARCH=arm64 go build -o dist/invest-log-backend ./cmd/server
```

## iOS / iPadOS (gomobile)

A gomobile wrapper is provided in `pkg/mobile`, which exposes JSON-based APIs that are friendly to gomobile bindings.

```bash
cd go-backend
# install gomobile (once)
# go install golang.org/x/mobile/cmd/gomobile@latest
# gomobile init

# build XCFramework
# gomobile bind -target=ios -o build/InvestLogCore.xcframework ./pkg/mobile
```

The resulting framework can be linked into a Swift/Obj-C app. The wrapper returns JSON strings for complex data (holdings, transactions, etc.), keeping bindings simple and stable across platforms.

## Migration Notes

- The Go backend auto-initializes/migrates the SQLite schema on startup.
- It preserves the same tables and core calculation rules (weighted average cost, cash linking, allocation warnings).
- Recommended: back up your existing `transactions.db` before first run.

## Price Fetching

Price fetching is ported with multi-source fallback and in-memory caching:
- Eastmoney (A-share / funds)
- Tencent Finance
- Sina Finance
- Yahoo Finance (US/HK + gold)

A simple circuit breaker is applied per source (3 failures in 60s → 120s cooldown).
