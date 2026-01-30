# Design - Invest Log

## 1. System Overview
Invest Log is a local web application and desktop app for tracking investment transactions and holdings. The backend is a FastAPI server with a SQLite database. The frontend uses Jinja2 templates, static assets, and Chart.js for charts. An Electron shell can package the app and launch a bundled backend binary.

## 2. Architecture
### 2.1 Backend (Python)
- Entry point: `app.py`.
- Framework: FastAPI with Jinja2 templates.
- Routers: feature-based modules under `routers/`.
- Data access and business logic: `database.py`.
- Price fetching: `price_fetcher.py`.
- Configuration: `config.py`.
- Logging: `logger_config.py`.

### 2.2 Frontend
- Server-rendered HTML in `templates/`.
- CSS and PWA assets in `static/`.
- Charting via Chart.js CDN.
- Privacy mode using localStorage to mask numeric values.

### 2.3 Desktop Packaging
- Electron main process in `electron/main.js`.
- Python backend bundled via PyInstaller (`invest-log-backend.spec`).
- Build script: `scripts/build.sh`.
- Packaging with `electron-builder` using `extraResources` to embed the backend binary.

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
- Symbol types are detected by format and currency (A-share, HK, US, fund, gold, cash, bond).
- Fallback sequence varies by symbol type. Examples:
  - A-share: pqquotation -> Tencent -> Sina -> Eastmoney -> Eastmoney fund -> Yahoo
  - HK/US: Yahoo -> Sina -> Tencent
  - Fund: multiple Eastmoney endpoints
  - Gold: Yahoo
- Caching:
  - 30-second TTL per (symbol, currency, asset_type).
- Circuit breaker:
  - After 3 failures in 60 seconds, a service cools down for 120 seconds.

## 6. Routes and Interfaces
### 6.1 HTML Pages
- GET `/` overview with allocations by currency.
- GET `/charts` portfolio charts.
- GET `/holdings` holdings detail.
- GET `/symbol/{symbol}` symbol detail and transactions.
- GET `/transactions` transaction list with pagination.
- GET `/add` add transaction form.
- GET `/settings` settings UI with tabs.
- GET `/setup` first-run setup page.

### 6.2 Actions (POST)
- POST `/add` add a transaction.
- POST `/holdings/update-price` fetch and store latest price.
- POST `/holdings/manual-update-price` manual price update.
- POST `/holdings/update-all` update all symbols for a currency.
- POST `/holdings/quick-trade` add a quick trade from holdings.
- POST `/holdings/toggle-auto-update` update symbol auto_update flag.
- POST `/holdings/update-asset-type` update symbol asset_type.
- POST `/settings` save allocation settings.
- POST `/settings/database` update DB file name and iCloud preference.
- POST `/settings/add-account` add account (+ initial balances).
- POST `/settings/delete-account/{account_id}` delete account if unused.
- POST `/settings/add-asset-type` add asset type.
- POST `/settings/delete-asset-type/{code}` delete asset type if unused.
- POST `/settings/update-symbol` update symbol metadata.
- POST `/setup/complete` complete setup via API.

### 6.3 JSON API
- GET `/api/health` backend readiness check (used by Electron).
- GET `/api/holdings` current holdings.
- GET `/api/holdings-by-currency` allocations and warnings.
- GET `/api/transactions` transactions with filters.
- GET `/api/portfolio-history` cumulative BUY/SELL flow over time.
- DELETE `/api/transactions/{id}` delete a transaction.

## 7. Configuration and Startup
- Config priority:
  1) Runtime CLI args (`--data-dir`, `--port`).
  2) Environment variables (`INVEST_LOG_DATA_DIR`, `INVEST_LOG_DB_PATH`).
  3) Config file in app config directory (or legacy project config.json).
  4) Platform defaults (iCloud on macOS if enabled, otherwise app config dir).
- First-run setup persists `setup_complete`, data dir, and DB name in config.
- Backend runs on 127.0.0.1 by default with uvicorn.
- Optional parent process watcher for desktop sidecar mode.

## 8. Desktop App Flow
- Electron selects a free local port (prefers 8000).
- Electron spawns the backend with `--data-dir` and `--port`.
- It polls `/api/health` until the backend is ready, then loads the web UI.
- On app quit, Electron terminates the backend process.

## 9. Logging and Audit
- Logs rotate daily with 7-day retention.
- Log directory is under the configured data dir.
- Operation logs in the database record price updates and manual actions.

## 10. Constraints and Risks
- SQLite write concurrency is limited; multi-instance writes can lock the DB.
- Price sources are best-effort and depend on external availability.
- No authentication layer; intended for local single-user use.
- Chart.js is loaded from a CDN and requires network access.
