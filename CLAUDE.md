# CLAUDE.md

This file provides guidance to Codex CLI when working with code in this repository.

## Project Overview

Invest Log is a local-first investment portfolio tracker.

- Backend: Go service in `go-backend/` (SQLite + JSON API, can serve the SPA).
- Frontend: SPA in `static/` (vanilla JS/CSS/HTML).
- Mobile (optional): iOS wrapper in `ios/` and Go mobile bindings in `go-backend/pkg/mobile`.

Key features:
- Multi-currency (CNY, USD, HKD) and multi-asset tracking
- Transaction types: BUY, SELL, DIVIDEND, SPLIT, TRANSFER_IN, TRANSFER_OUT, ADJUST, INCOME
- Weighted average cost basis
- Price updates with multi-source fallback and caching
- Allocation monitoring with min/max thresholds

## Development Commands

### Run the Go backend

```bash
cd go-backend
go run ./cmd/server --data-dir /path/to/data --port 8000
```

Optional flags:
- `--web-dir`: path to SPA assets (defaults to `static` or `../static` if found)

Environment variables:
- `INVEST_LOG_DATA_DIR`: override data directory
- `INVEST_LOG_DB_PATH`: override database file path

### Build a native backend binary (macOS example)

```bash
cd go-backend
mkdir -p dist
GOOS=darwin GOARCH=arm64 go build -o dist/invest-log-backend ./cmd/server
```

### SPA usage

- The Go server serves the SPA at `/` when `--web-dir` points to `static/`.
- If opening `static/index.html` directly (file or mobile wrapper), set the API base in Settings or pass `?api=http://127.0.0.1:8000`.

### iOS wrapper (Capacitor)

- Sync the SPA into the iOS project with:
  - `scripts/sync_spa.sh` (macOS/Linux)
  - `scripts/sync_spa.ps1` (Windows)

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

## Data Model (SQLite)

Key tables:
- `transactions`, `accounts`, `symbols`, `allocation_settings`, `asset_types`,
  `operation_logs`, `latest_prices`

## Business Rules

- Weighted average cost basis (cost ÷ shares) per symbol and currency.
- CASH holdings are treated as balance with price fixed at 1.0.
- When cash linking is enabled, BUY/SELL auto-create matching CASH transactions.

## Price Fetching

Sources (with fallback): Eastmoney, Tencent, Sina, Yahoo Finance.
A simple circuit breaker is applied per source (3 failures in 60s -> 120s cooldown).

## Logging

Logs are written under the data directory in `logs/` with daily rotation (7 days).

## Repo Structure (high level)

```
invest-log/
├── go-backend/        # Go API + static server
├── static/            # SPA assets
├── ios/               # Capacitor iOS project
├── scripts/           # Utility scripts (sync SPA)
└── output/            # Build artifacts (ignored)
```
