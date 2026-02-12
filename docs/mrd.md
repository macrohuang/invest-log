# MRD - Invest Log

## 1. Purpose
Invest Log is a local-first investment portfolio tracking system. It records transactions, calculates holdings and cost basis, tracks prices, and visualizes allocations across currencies and asset types. It runs as a local web app powered by a Go backend and a SPA, with recent capabilities to leverage AI for portfolio analysis.

## 2. Target Users
- Individual investors managing personal portfolios across multiple broker accounts.
- Users who prioritize local data privacy and storage ownership.
- Users who need multi-currency and multi-asset tracking without relying on automated broker integrations.
- Investors seeking objective, theory-based analysis of their portfolio structure.

## 3. Goals
- Make it fast to record investment transactions, transfers, and cash movements.
- Provide accurate holdings, cost basis, and PnL using weighted average methods.
- Track allocations by currency and asset type with customizable warning thresholds.
- Enable on-demand and manual price updates per symbol, including exchange rates.
- Provide intelligent portfolio diagnosis using AI models (OpenAI compatible).
- Ship a lightweight, portable backend as a native Go binary.

## 4. Scope

### In Scope
- **Core Tracking**: Multi-currency (CNY, USD, HKD) and multi-asset (stock, bond, fund, metal, cash, etc.) tracking.
- **Transactions**: comprehensive logging: BUY, SELL, DIVIDEND, SPLIT, TRANSFER_IN, TRANSFER_OUT, ADJUST, INCOME, TRANSFER (cross-account).
- **Account & Storage**: Account management; Database file management (switch/create databases at runtime).
- **Symbol Management**: Metadata editing, asset type classification, auto-update toggles, and fast frontend filtering.
- **Holdings & PnL**: Real-time calculation of weighted average cost, PnL, and market value.
- **Market Data**: Multi-source price fetching (fallback support) and manual exchange rate management.
- **Analytics**:
    - Allocation charts (Pie/Donut) by currency and symbol.
    - Portfolio history visualization.
    - **AI Analysis**: Portfolio diagnosis based on investment theories (Malkiel, Dalio, Buffett).
- **System**:
    - JSON API for all data operations.
    - Operation logs for audit trails.
    - PWA support (offline capabilities for static assets).
    - Mobile wrapper compatibility.

### Out of Scope
- Direct broker API connections for automated syncing.
- Real-time streaming quotes (tick-level data).
- Multi-user SaaS architecture (strictly single-user local).
- Complex tax lot tracking (FIFO/LIFO specific logic) - Weighted Average is default.
- Automated algorithmic trading.

## 5. Core User Journeys
1. **Setup**: Configure storage location, start server, or switch to a different database file via UI.
2. **Onboarding**: Add accounts, define initial cash balances, and configure asset types.
3. **Recording**: Log trades, dividends, or transfer assets/cash between accounts.
4. **Monitoring**: View holdings, refresh market prices/exchange rates, and check allocation warnings.
5. **Analysis**:
    - Use "AI Analyze" to get a structural diagnosis of the portfolio.
    - View visual charts to understand exposure.
6. **Maintenance**: Adjust cost basis if needed, manage symbol properties using quick filters.

## 6. Functional Requirements

### FR-1 Setup and Storage Management
- Allow configuration via CLI flags/Env.
- **New**: Support switching database files at runtime via the Settings UI.
- **New**: Create new database files directly from the UI.

### FR-2 Accounts
- Create/Edit/Delete accounts with broker and type metadata.
- Support initial balance setup.

### FR-3 Transactions & Transfers
- Record standard trade types.
- **New**: Support `TRANSFER` transactions for moving assets/cash between accounts (pairs `TRANSFER_OUT` and `TRANSFER_IN`).
- Link BUY/SELL to CASH transactions automatically.

### FR-4 Holdings and Cost Basis
- Calculate weighted average cost basis.
- Real-time aggregation by Symbol, Currency, and Account.

### FR-5 Price & Exchange Rates
- Fetch symbol prices from multiple sources (Eastmoney, Yahoo, Sina, etc.).
- **New**: Manage and refresh currency Exchange Rates (e.g., USD -> CNY) for unified valuation.
- Support manual price overrides.

### FR-6 Allocation Monitoring
- Set min/max targets per Currency + Asset Type.
- Visual warnings for drift.

### FR-7 Charts
- Allocation breakdown by currency and symbol.
- Portfolio Net Worth history.

### FR-8 Symbol Management
- Edit symbol metadata (Name, Sector, Exchange).
- **New**: Quick Filter in UI to search symbols by code, name, or type instantly.

### FR-9 API
- RESTful JSON API for all frontend interactions.

### FR-10 Packaging
- Native Go binary (macOS/Linux/Windows).
- Embeddable SPA.

### FR-11 Logging
- Application logs (system).
- Operation logs (user actions: price updates, manual edits).

### FR-12 PWA & Privacy
- Web Manifest + Service Worker.
- Privacy Mode (mask sensitive figures).

### FR-13 AI Holdings Analysis
- **New**: One-click portfolio analysis using OpenAI-compatible APIs.
- User-configurable Model, API Key (stored locally), and Base URL.
- Structured output: Summary, Risk Level, Key Findings, and Actionable Recommendations.

## 7. Non-Functional Requirements
- **Local-first**: Data lives in SQLite; API Keys for AI are stored in browser `localStorage`.
- **Performance**: <200ms API response for core reads; <20s for AI analysis.
- **Resilience**: Circuit breakers for external price APIs.
- **Compatibility**: Responsive web design (Desktop + Mobile).

## 8. Constraints and Assumptions
- Single user concurrency (SQLite limitation).
- AI Analysis is advisory only; no guarantee of financial returns.
- Exchange rates are fetched or set manually; no real-time forex streaming.

## 9. Success Metrics
- Setup time < 5 mins.
- AI Analysis returns valid JSON structure > 95% of the time.
- Price fetching success rate > 90% for supported markets (A-share, HK, US).
