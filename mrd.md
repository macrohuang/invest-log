# MRD - Invest Log

## 1. Purpose
Invest Log is a local-first investment portfolio tracking system. It records transactions, calculates holdings and cost basis, tracks prices, and visualizes allocations across currencies and asset types. It runs as a local web app powered by a Go backend and a SPA.

## 2. Target Users
- Individual investors managing personal portfolios across multiple broker accounts.
- Users who want local data storage.
- Users who need multi-currency and multi-asset tracking without broker integrations.

## 3. Goals
- Make it fast to record investment transactions and cash movements.
- Provide accurate holdings, cost basis, and PnL using weighted average.
- Track allocations by currency and asset type with warning thresholds.
- Enable on-demand and manual price updates per symbol.
- Ship a lightweight backend as a native Go binary.

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
- JSON API for health, holdings, transactions, and portfolio history.
- Operation logging for price updates and manual changes.
- PWA assets (manifest + service worker) and privacy mode in UI.
- Optional mobile wrapper that embeds the SPA.

### Out of Scope
- Broker connections and live trading.
- Real-time streaming quotes.
- Authentication and multi-user access control.
- Tax reporting and compliance workflows.
- Automated rebalancing or trading recommendations.

## 5. Core User Journeys
1. Configure storage location (via flags/env) and start the local server.
2. Add accounts with optional initial cash balances per currency.
3. Record transactions, optionally linking to a cash movement.
4. Review holdings, update prices, and inspect symbol details.
5. Adjust asset values when needed via an ADJUST transaction.
6. Configure allocation targets and monitor warnings.
7. View charts and allocations; toggle privacy mode for display.

## 6. Functional Requirements

### FR-1 Setup and Storage
- Allow users to configure data storage via CLI flags or environment variables.
- Support pointing to an existing database file path.

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

### FR-9 API
- Provide JSON endpoints for health, holdings, holdings-by-currency, transactions, and portfolio history.
- Support deletion of transactions via API.

### FR-10 Packaging and Portability
- Build the backend as a native Go binary for desktop platforms.
- Serve the SPA from the backend or embed it in a mobile wrapper.

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
- Portability: run as a local web server via a Go binary across desktop platforms.

## 8. Constraints and Assumptions
- SQLite supports only limited concurrent writes; avoid multiple writers.
- Supported currencies are limited to CNY, USD, and HKD.
- Bonds are not automatically priced; cash price is fixed at 1.0.

## 9. Success Metrics
- User can complete setup and add the first transaction in under 5 minutes.
- Holdings and allocation views reflect new transactions immediately.
- Price updates succeed for common symbols with at least one data source.
- The local server starts and serves the UI within 10 seconds on typical hardware.
