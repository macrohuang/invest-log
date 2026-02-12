# Design - Invest Log

## 1. System Overview
Invest Log is a comprehensive, local-first investment tracking solution. It uses a Go backend (`chi` router + `sqlite`) to serve a pure JavaScript SPA. The system emphasizes data privacy, ownership, and rapid interaction. It now includes AI-powered capabilities for portfolio analysis and supports runtime database management.

## 2. Architecture

### 2.1 Backend (Go)
- **Core**: `go-backend/pkg/investlog` contains the domain logic, database interaction, and calculation engines.
- **API Layer**: `go-backend/internal/api` handles HTTP requests, validation, and JSON marshaling.
- **Storage**: SQLite with connection pooling (max 1 writer). Supports runtime hot-swapping of database files.
- **AI Integration**: Acts as a proxy/client to OpenAI-compatible services, sanitizing inputs and parsing structured outputs.

### 2.2 Frontend (SPA)
- **Tech Stack**: Vanilla JS (ES6+), CSS3 variables, Hash Router. No build step required for dev.
- **State**: Local state for data; `localStorage` for UI preferences (Privacy Mode, AI Config, Theme).
- **PWA**: `sw.js` handles caching for offline shell access.

### 2.3 Mobile/Cross-Platform
- **iOS**: Capacitor project (`ios/`) wrapping the SPA.
- **Desktop**: Go binary serving local HTTP.

## 3. Data Model

### 3.1 Core Tables
- `accounts`: Stores wallet/brokerage accounts.
    - `id, name, broker, type, currency, ...`
- `symbols`: Metadata for tracked assets.
    - `id, symbol, name, asset_type, sector, exchange, auto_update`
- `transactions`: The ledger of all events.
    - `id, date, type, symbol_id, quantity, price, amount, commission, account_id, linked_transaction_id (for transfers/cash-link)`
- `allocations_settings`: Target percentages.
    - `currency, asset_type, min_pct, max_pct`
- `exchange_rates`: **(New)** Stores conversion rates between currencies.
    - `from_currency, to_currency, rate, update_type, updated_at`
- `latest_prices`: Most recent market data.
    - `symbol, currency, price, updated_at`
- `operation_logs`: Audit trail.
- `asset_types`: Configurable categories (e.g., Stock, REIT, Gold).

### 3.2 Key Relationships
- `transactions` -> `symbols` (Many-to-One)
- `transactions` -> `accounts` (Many-to-One)
- `transactions` self-referencing via `linked_transaction_id` (One-to-One, for Transfers or Trade-to-Cash links).

## 4. Core Business Logic

### 4.1 Transaction Processing
- **Weighted Average Cost**: Updated dynamically on every transaction fetch/calculation.
- **Transfers**: Handled as a pair of `TRANSFER_OUT` (Source Account) and `TRANSFER_IN` (Dest Account), linked by ID.
- **Cash Linking**: Automated creation of counterpart Cash transaction when buying/selling assets.

### 4.2 Storage Management (New)
- **Hot-Swap**: The `Core` struct can be re-initialized with a different DB path at runtime.
- **Locking**: Uses `sync.RWMutex` to protect the `Core` reference during switches to ensure active requests complete.

### 4.3 AI Analysis (New)
- **Input**: Aggregates current holdings (grouped by symbol) + User Config (Risk Profile, Horizon).
- **Process**: Constructs a prompt asking for JSON output based on specific investment philosophies (Malkiel/Dalio/Buffett).
- **Output**: Returns structured `AnalysisResult` (Summary, Findings, Recommendations) directly to frontend.
- **Security**: API Keys are passed from frontend per request, never stored in DB.

### 4.4 Price & Exchange Rates
- **Market Data**: Fetcher attempts sources in priority order (Eastmoney -> Yahoo -> Sina -> etc.).
- **Exchange Rates**:
    - Can be updated manually or fetched.
    - Used to normalize portfolio value into a single base currency for total aggregation.

## 5. Interfaces & API

### 5.1 System & Storage
- `GET /api/health`: Status check.
- `GET /api/storage/info`: List available DBs, current DB path.
- `POST /api/storage/switch`: Switch active database or create new one.

### 5.2 Investment Data
- `GET /api/holdings`: Aggregated view.
- `GET /api/transactions`: List with filtering (paged).
- `POST /api/transactions`: Add trade.
- `POST /api/transfers`: **(New)** Execute account transfer.
- `GET /api/portfolio-history`: Historical net worth.
- `GET /api/accounts`, `POST /api/accounts`
- `GET /api/symbols`, `PUT /api/symbols/{id}`

### 5.3 AI & Analytics
- `POST /api/ai/holdings-analysis`: Trigger portfolio diagnosis.

### 5.4 Market Data
- `GET /api/exchange-rates`: List current rates.
- `POST /api/exchange-rates`: Set manual rate.
- `POST /api/exchange-rates/refresh`: Fetch latest rates.
- `POST /api/prices/update`: Update single symbol.
- `POST /api/prices/update-all`: Batch update.

## 6. Security & Safety
- **Concurrency**: SQLite WAL mode enabled (via `modernc.org/sqlite` config) for better concurrency.
- **Validation**: Strict JSON decoding (`DisallowUnknownFields`), enum checks for transaction types.
- **Error Handling**: 
    - Transactions wrapped in `db.Begin/Commit`.
    - Panic recovery in `WithTx` helper (logs error before re-panic).
    - Resource cleanup (Rows, Statements) enforced via `defer`.

## 7. Configuration
- **Runtime**: `data-dir`, `port` flags.
- **Dynamic**: Database selection is now dynamic, persisting preference in `config.json`.

## 8. Constraints
- **Latency**: AI analysis is synchronous; client timeout set to ~60s to accommodate model generation time.
- **Mobile**: iOS wrapper relies on `WKWebView` limitations; local Go server runs as a background process on mobile (if using native binding approach) or connecting to remote/local-network server.
