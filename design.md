# Design - Invest Log

## 1. System Overview
Invest Log is a local-first investment portfolio tracker. A Go backend manages data and business rules, while a SPA provides the user interface. The backend can serve the SPA directly or the SPA can be hosted by a mobile wrapper.

## 2. Architecture

### 2.1 Backend (Go)
- Entry point: `go-backend/cmd/server`.
- SQLite for persistence (same schema as legacy implementation).
- JSON API for all operations.
- Optional static file server for the SPA (`--web-dir`).
- Configuration via CLI flags and environment variables.

### 2.2 Frontend (SPA)
- Static assets in `static/` (HTML, CSS, JS).
- Hash-based routing for views (overview, holdings, charts, transactions, settings).
- Calls the JSON API for all data and actions.
- PWA assets (manifest + service worker).

### 2.3 Mobile (optional)
- Capacitor iOS project under `ios/` consumes the SPA assets.
- Go mobile bindings under `go-backend/pkg/mobile` expose JSON-friendly APIs for native wrappers.
- `scripts/sync_spa.sh` and `scripts/sync_spa.ps1` sync `static/` into the iOS project.

## 3. Data Model

### 3.1 Tables
- `accounts`
  - account_id (PK), account_name, broker, account_type, created_at
- `symbols`
  - id (PK), symbol (unique), name, asset_type, sector, exchange, auto_update
- `transactions`
  - id (PK), transaction_date, transaction_time, symbol_id (FK), transaction_type,
    quantity, price, total_amount, commission, currency, account_id, account_name,
    notes, tags, created_at, updated_at
- `allocation_settings`
  - id (PK), currency, asset_type, min_percent, max_percent
- `asset_types`
  - id (PK), code (unique), label, created_at
- `operation_logs`
  - id (PK), operation_type, symbol, currency, details, old_value, new_value, price_fetched, created_at
- `latest_prices`
  - id (PK), symbol, currency, price, updated_at (unique per symbol+currency)

### 3.2 Indexes
- transactions: symbol_id, transaction_date, account_id, transaction_type, currency
- symbols: asset_type

## 4. Core Business Logic

### 4.1 Holdings Calculation
- Aggregate transactions by symbol, currency, and account.
- total_shares:
  - BUY, TRANSFER_IN, INCOME add quantity
  - SELL, TRANSFER_OUT subtract quantity
  - SPLIT, ADJUST add quantity (ADJUST uses quantity=0 by default)
- total_cost:
  - BUY and INCOME add (total_amount + commission)
  - SELL subtract (total_amount - commission)
  - ADJUST adds total_amount override
- avg_cost = total_cost / total_shares when shares > 0
- CASH holdings are treated as balance with price fixed at 1.0

### 4.2 Cash Linking
- When `link_cash` is enabled for BUY/SELL:
  - BUY creates a SELL transaction for CASH with amount = total_amount + commission.
  - SELL creates a BUY transaction for CASH with amount = total_amount - commission.

### 4.3 Asset Value Adjustment
- ADJUST creates a transaction that changes cost basis without changing shares.
- new_value - current_value is stored in total_amount (override) and price field.

### 4.4 Allocation Monitoring
- Allocation per (currency, asset_type) is computed from market value.
- Market value uses latest_prices if available, otherwise cost basis.
- Warnings are triggered when percent < min or > max for the configured range.

## 5. Price Fetching Subsystem
- Multi-source fallback: Eastmoney, Tencent Finance, Sina Finance, Yahoo Finance.
- Caching: in-memory with short TTL per symbol.
- Circuit breaker: 3 failures in 60 seconds -> 120 seconds cooldown per source.

## 6. Interfaces

### 6.1 SPA Views (client routes)
- `#/overview`
- `#/holdings`
- `#/charts`
- `#/transactions`
- `#/settings`

### 6.2 JSON API
- `GET /api/health`
- `GET /api/holdings`
- `GET /api/holdings-by-currency`
- `GET /api/holdings-by-symbol`
- `GET /api/transactions`
- `POST /api/transactions`
- `DELETE /api/transactions/{id}`
- `GET /api/portfolio-history`
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

## 7. Configuration and Startup
- CLI flags: `--data-dir`, `--port`, `--web-dir`.
- Environment variables: `INVEST_LOG_DATA_DIR`, `INVEST_LOG_DB_PATH`.
- Default server binds to 127.0.0.1 unless overridden.

## 8. Logging and Audit
- Logs rotate daily with 7-day retention.
- Log directory is under the configured data directory.
- Operation logs in the database record price updates and manual actions.

## 9. Constraints and Risks
- SQLite write concurrency is limited; avoid multiple writers.
- Price sources are best-effort and depend on external availability.
- No authentication layer; intended for local single-user use.
