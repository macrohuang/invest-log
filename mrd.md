# MRD - Invest Log

## 1. Purpose
Invest Log is a local-first investment portfolio tracking system. It records transactions, calculates holdings and cost basis, tracks prices, and visualizes allocations across currencies and asset types. It can run as a local web app or a packaged desktop app.

## 2. Target Users
- Individual investors managing personal portfolios across multiple broker accounts.
- Users who want local data storage with optional iCloud sync on macOS.
- Users who need multi-currency and multi-asset tracking without broker integrations.

## 3. Goals
- Make it fast to record investment transactions and cash movements.
- Provide accurate holdings, cost basis, and PnL using weighted average.
- Track allocations by currency and asset type with warning thresholds.
- Enable on-demand and manual price updates per symbol.
- Support desktop packaging with a bundled backend.

## 4. Scope
### In Scope
- Multi-currency tracking (CNY, USD, HKD).
- Multi-asset tracking with configurable asset types (default: stock, bond, metal, cash).
- Transaction logging with the following types: BUY, SELL, DIVIDEND, SPLIT, TRANSFER_IN, TRANSFER_OUT, ADJUST, INCOME.
- Account management (create, list, delete if unused).
- Symbol management (name, asset type, auto-update toggle).
- Holdings views by currency and by symbol with allocation and PnL metrics.
- Price fetching from multiple sources with fallback and caching; manual price override.
- Allocation settings per (currency, asset_type) with min/max warning thresholds.
- Charts for portfolio breakdown per currency and per symbol.
- First-run setup for data directory selection and optional iCloud sync (macOS).
- Desktop app integration via Electron with a bundled Python sidecar.
- Basic JSON API for health, holdings, transactions, and portfolio history.
- Operation logging for price updates and manual changes.
- PWA assets (manifest + service worker) and privacy mode in UI.

### Out of Scope
- Broker connections and live trading.
- Real-time streaming quotes.
- Authentication and multi-user access control.
- Tax reporting and compliance workflows.
- Automated rebalancing or trading recommendations.

## 5. Core User Journeys
1. First run setup: choose storage location (local, iCloud on macOS, or custom), optionally attach an existing database.
2. Add accounts with optional initial cash balances per currency.
3. Record transactions, optionally linking to a cash movement.
4. Review holdings, update prices, and inspect symbol details.
5. Adjust asset values when needed via an ADJUST transaction.
6. Configure allocation targets and monitor warnings.
7. View charts and allocations; toggle privacy mode for display.

## 6. Functional Requirements
### FR-1 Setup and Storage
- Allow users to select data storage at first run (local, iCloud on macOS, or custom path).
- Support attaching an existing database file and copying it to the chosen location.
- Persist setup in a config file and allow later changes in settings.

### FR-2 Accounts
- Create accounts with optional broker/type metadata.
- Prevent deletion of accounts that have transactions.
- Optionally create initial cash balances via TRANSFER_IN transactions.

### FR-3 Transactions
- Record transactions with date, symbol, type, quantity, price, commission, currency, account, and notes.
- Support linking BUY/SELL to a corresponding CASH transaction with commission handling.
- Provide list view with pagination and delete capability.

### FR-4 Holdings and Cost Basis
- Calculate holdings per symbol and currency using weighted average cost.
- Track total shares, cost basis, average cost, and PnL when prices are available.
- Treat CASH holdings as a balance (price fixed at 1.0).

### FR-5 Price Updates
- Fetch latest prices on demand with multiple data sources and fallback.
- Cache prices for a short TTL to reduce repeated calls.
- Allow manual price updates per symbol.
- Store latest prices in a dedicated table with timestamps.

### FR-6 Allocation Monitoring
- Configure min/max allocation ranges per currency and asset type.
- Compute allocation percentages and show warnings when outside ranges.

### FR-7 Charts and Analytics
- Display currency-level allocation charts.
- Display per-symbol charts within each currency, grouped by account for legend clarity.
- Provide portfolio history based on BUY and SELL cash flow.

### FR-8 Settings and Metadata
- Manage asset types (create, delete if unused).
- Manage symbol metadata (name, asset type, auto-update toggle).
- Configure database file name and iCloud sync option.

### FR-9 API
- Provide basic JSON endpoints for health, holdings, holdings-by-currency, transactions, and portfolio history.
- Support deletion of transactions via API.

### FR-10 Desktop App
- Bundle a Python backend sidecar and launch it from Electron.
- Dynamically choose a free local port, wait for health check, then load the UI.
- Shutdown the backend when the desktop app exits.

### FR-11 Logging
- Log application operations with daily rotation and retention.
- Record price update operations and manual changes in operation logs.

### FR-12 PWA and Privacy UI
- Provide a web manifest and service worker for caching static assets.
- Offer a privacy mode toggle that masks sensitive numbers in the UI.

## 7. Non-Functional Requirements
- Local-first: data stored in SQLite on the user machine.
- Performance: typical views should render within a few seconds for hundreds of records.
- Reliability: multi-source price fetching with fallback and circuit breaker behavior.
- Data integrity: parameterized SQL queries and controlled transaction types.
- Security: intended for local use without authentication; rely on OS file permissions.
- Portability: run as a local web server and package for desktop on macOS, Windows, and Linux.

## 8. Constraints and Assumptions
- SQLite supports only limited concurrent writes; avoid multiple writers.
- Supported currencies are limited to CNY, USD, and HKD.
- Bonds are not automatically priced; cash price is fixed at 1.0.
- Chart.js is loaded from a CDN and requires network access.

## 9. Success Metrics
- User can complete setup and add the first transaction in under 5 minutes.
- Holdings and allocation views reflect new transactions immediately.
- Price updates succeed for common symbols with at least one data source.
- Desktop app launches the backend and renders the UI within 60 seconds.
