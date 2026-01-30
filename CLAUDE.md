# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Invest Log is a personal investment portfolio management system built with FastAPI (Python backend), SQLite database, and Jinja2 templates. It supports multi-currency (CNY, USD, HKD) and multi-asset (stocks, bonds, metals, cash) tracking with real-time price fetching from multiple data sources. The application can run as a web server or be packaged as a desktop app using Electron.

**Key Features:**
- Multi-currency portfolio management with automatic currency grouping
- Comprehensive transaction types: BUY, SELL, DIVIDEND, SPLIT, TRANSFER_IN, TRANSFER_OUT, ADJUST, INCOME
- Intelligent cash linking for automatic cash flow tracking
- Cost basis calculation using weighted average method
- Real-time price updates from multiple data sources (AKShare, Yahoo Finance, Sina, Tencent)
- Asset allocation monitoring with configurable min/max ranges
- iCloud synchronization for data backup (macOS)

## Development Commands

### Running the Application

**Development server:**
```bash
python app.py
# Defaults to http://127.0.0.1:8000
# Use --port and --host flags to customize
```

**With custom data directory:**
```bash
python app.py --data-dir /path/to/data --port 8080
```

**Using uvicorn directly:**
```bash
uvicorn app:app --reload --host 127.0.0.1 --port 8000
```

### Desktop App Development

**Build Python sidecar binary:**
```bash
./scripts/build.sh sidecar
# Or: pyinstaller invest-log-backend.spec
```

**Run in development mode (Electron):**
```bash
./scripts/build.sh dev
# Or: npm run dev (after building sidecar)
```

**Build release bundle (Electron):**
```bash
./scripts/build.sh release
# Output: electron-builder output (default: dist/)
```

### Dependency Management

**Install runtime dependencies:**
```bash
pip install -r requirements.txt
```

**Install development dependencies (includes PyInstaller):**
```bash
pip install -r requirements-dev.txt
```

## Architecture Overview

### Three-Layer Architecture

1. **Application Layer** (`app.py`)
   - FastAPI application initialization with lifespan management
   - Static file mounting and router registration
   - CLI argument parsing for desktop app mode
   - Signal handlers for graceful shutdown

2. **Service Layer**
   - **Database Module** (`database.py`): SQLite operations with connection pooling, transaction management, and business logic aggregations
   - **Price Fetcher** (`price_fetcher.py`): Multi-source price fetching with fallback chain (AKShare → Yahoo Finance → Sina → Tencent)
   - **Config Module** (`config.py`): Centralized configuration with support for iCloud sync, custom data directories, and runtime overrides

3. **Presentation Layer**
   - **Routers** (`routers/`): Feature-based routing modules
   - **Templates** (`templates/`): Jinja2 HTML templates with base template inheritance
   - **Static Files** (`static/`): CSS and client-side assets

### Router Organization

Each router handles a specific domain:

- `overview.py`: Dashboard and portfolio summary with charts
- `holdings.py`: Holdings detail, price updates, symbol detail pages, asset value adjustments
- `transactions.py`: Transaction list with pagination, add transaction form
- `settings.py`: Allocation settings, asset types, account management
- `api.py`: REST API endpoints for holdings, transactions, portfolio history
- `setup.py`: First-run setup wizard for data directory configuration

### Database Schema

Key tables in SQLite database:

- `transactions`: Core transaction log (buy/sell/dividend/split/transfer/adjust/income)
- `accounts`: Account metadata (account_id, broker, account_type)
- `symbols`: Symbol information (name, asset_type, sector, exchange)
- `allocation_settings`: Target allocation ranges per currency and asset type
- `asset_types`: Asset type definitions (stock/bond/metal/cash)
- `operation_logs`: Audit trail for price updates and adjustments
- `latest_prices`: Cached latest prices per symbol and currency

Indexes exist on: symbol, transaction_date, account_id, transaction_type, currency, asset_type

## Configuration System

Configuration priority (highest to lowest):

1. **Runtime overrides**: CLI arguments (`--data-dir`, `--port`)
2. **Environment variables**: `INVEST_LOG_DATA_DIR`, `INVEST_LOG_DB_PATH`
3. **Config file**: `~/Library/Application Support/InvestLog/config.json` (macOS) or project-local `config.json`
4. **Defaults**: iCloud Drive on macOS (`~/Library/Mobile Documents/com~apple~CloudDocs/InvestLog/`) or local app config directory

Use `config.complete_setup()` to finalize first-run configuration.

## Price Fetching System

### Multi-Source Fallback Strategy

The price fetcher attempts multiple data sources in order:

1. **AKShare** (primary): Best for A-shares and Chinese markets
2. **Yahoo Finance** (yfinance): Reliable for US/HK stocks
3. **Sina Finance API**: Backup for A-shares
4. **Tencent Finance API**: Backup for A-shares and HK stocks

### Symbol Type Detection

Automatic detection based on format and currency:
- A-shares: `SH600000`, `SZ000001`, or 6-digit codes with CNY
- HK stocks: 5-digit codes starting with 0 (e.g., `00700`) with HKD
- US stocks: Alphabetic symbols with USD
- Gold: Symbols containing "AU" or "GOLD"
- Cash: Symbol "CASH"
- Bonds: Symbols containing "BOND"

When adding price fetching logic, always follow the fallback pattern and log each attempt.

## Common Development Tasks

### Adding a New Transaction Type

1. Update the CHECK constraint in `database.py` (already includes: BUY, SELL, DIVIDEND, SPLIT, TRANSFER_IN, TRANSFER_OUT, ADJUST, INCOME)
2. Add handling logic in relevant routers (typically `transactions.py` and `holdings.py`)
3. Update business logic in `database.py` aggregation functions if needed

### Adding a New Currency or Asset Type

Currencies and asset types are defined via CHECK constraints in the transactions table. To add new ones:

1. Create a migration in `database.py` to update the CHECK constraint
2. Update the asset_types table with new entries via `database.add_asset_type()`
3. Update frontend forms in templates to include new options

### Extending Price Data Sources

To add a new price data source:

1. Add fetcher function in `price_fetcher.py` following naming pattern `{source}_fetch_{symbol_type}()`
2. Update `fetch_price()` to include the new source in the fallback chain
3. Handle import errors gracefully (see existing try/except blocks for akshare/yfinance)
4. Log attempts and results for debugging

## Desktop App Integration

### Electron Configuration

The app uses Electron for desktop packaging:

- Main process: `electron/main.js`
- Loading screen: `electron/loading.html`
- Build config: `package.json` (electron-builder)
- Sidecar binary: Python backend packaged with PyInstaller
- Sidecar path: `dist/invest-log-backend-{platform}-{arch}`
- Default window: 1200x800, minimum 800x600

### Platform-Specific Binary Naming

PyInstaller generates platform-specific binaries:
- macOS (ARM): `invest-log-backend-aarch64-apple-darwin`
- macOS (x86): `invest-log-backend-x86_64-apple-darwin`
- Windows: `invest-log-backend-x86_64-pc-windows-msvc.exe`
- Linux: `invest-log-backend-x86_64-unknown-linux-gnu`

The build script (`scripts/build.sh`) handles detection and proper naming.

## Important Implementation Notes

### Database Connections

- Each operation opens and closes its own connection
- Use `get_connection()` for all database access
- Row factory is enabled by default (returns dict-like rows)
- All write operations should commit before closing
- No long-lived connections or connection pooling (SQLite limitation)
- **CRITICAL**: SQLite has limited concurrent write capability. Multiple instances writing simultaneously can cause database locking issues.

### Business Rules and Calculations

#### Cost Basis Calculation (Weighted Average Method)

The system calculates cost basis using weighted average:
```
Total Cost = Σ(quantity × price) for all BUY transactions
Total Shares = Σ(quantity) for all BUY transactions - Σ(quantity) for all SELL transactions
Average Cost = Total Cost ÷ Total Shares (if Total Shares > 0)
Unrealized P/L = (Current Price - Average Cost) × Total Shares
```

See `database.get_holdings_by_currency()` in database.py:344-390 for implementation.

#### Transaction Types and Portfolio Impact

| Transaction Type | Quantity Impact | Cost Basis Impact | Cash Flow |
|-----------------|-----------------|-------------------|-----------|
| BUY | +quantity | Increases total cost | Outflow (if linked) |
| SELL | -quantity | No change to avg cost | Inflow (if linked) |
| DIVIDEND | No change | No change | Inflow (separate record) |
| SPLIT | +quantity (ratio) | Proportionally reduces cost | None |
| TRANSFER_IN | +quantity | Adds to cost basis | None |
| TRANSFER_OUT | -quantity | No change to avg cost | None |
| ADJUST | No change | Adjusts cost only | None |
| INCOME | No change (for symbol "CASH") | N/A | Inflow |

#### Cash Linking Mechanism

When adding BUY/SELL transactions with `link_cash=True`:
- **BUY transaction**: Automatically creates a SELL transaction for symbol "CASH" with opposite amount (representing cash outflow)
- **SELL transaction**: Automatically creates a BUY transaction for symbol "CASH" with the received amount (cash inflow)
- Commission is deducted from the cash transaction
- Both transactions use the same account_id and currency

This maintains accurate cash position tracking. See `database.add_transaction()` in database.py:158-225.

#### Asset Allocation Monitoring

Users can set min/max percentage ranges for each (currency, asset_type) combination. The system:
1. Calculates current allocation as: `(asset_value / total_portfolio_value) × 100`
2. Compares against configured ranges
3. Displays warnings when allocation exceeds boundaries

This is visual only - no automatic rebalancing occurs.

### First-Run Setup

On first launch (no config file exists):
1. Check `config.is_first_run()`
2. Redirect to `/setup` route
3. User selects iCloud, custom directory, or default local storage
4. Call `config.complete_setup()` to persist configuration
5. Database initializes at selected location

### Logging

Log files are stored in `logs/` directory with daily rotation (7 day retention). Use the configured logger from `logger_config`:

```python
from logger_config import logger

logger.info("Operation completed")
logger.warning("Potential issue detected")
logger.error("Operation failed", exc_info=True)
```

## Code Standards and Best Practices

### Python Code Style

- Follow PEP 8 conventions (indentation, line length, naming)
- Function names: lowercase_with_underscores (e.g., `get_holdings`, `update_latest_price`)
- Constants: UPPERCASE_WITH_UNDERSCORES (e.g., `DB_PATH`)
- Provide docstrings for public functions explaining inputs, outputs, and side effects
- Import order: standard library → third-party → project modules (alphabetical within groups)

### Database Operations

**SQL Injection Prevention:**
- ALWAYS use parameterized queries, never string concatenation
- Example: `cursor.execute("SELECT * FROM transactions WHERE symbol = ?", (symbol,))`

**Connection Management:**
- Explicitly commit and close connections in write operations
- Consider using context managers (`with`) for future improvements
- Current pattern: `conn = get_connection()` → operations → `conn.commit()` → `conn.close()`

**Transaction Consistency:**
- Cash-linked transactions currently use multiple independent commits
- Consider wrapping related operations in a single transaction for atomicity
- Be aware that the current implementation could leave orphaned records on partial failures

**Query Optimization:**
- Use existing indexes (symbol, date, account_id, type, currency, asset_type)
- Prefer specific column selection over `SELECT *` for large tables
- Use pagination (LIMIT/OFFSET) for large result sets
- Monitor slow queries and add indexes as needed

### API Development

**Error Handling:**
- Return 404 for resources not found
- Return 422/400 for invalid parameters
- Log all errors with sufficient context
- Don't expose internal error details to clients

**Parameter Validation:**
- Validate all user inputs at route layer
- Check currency/asset_type against allowed enums
- Validate date formats and ranges
- Use Pydantic models for complex request validation (recommended for future)

**Response Format:**
- Maintain consistent JSON structure across endpoints
- Use snake_case for JSON field names
- Include appropriate HTTP status codes
- Document expected response format

### Security Checklist

**Input Validation:**
- Whitelist validation for symbol, account_id, currency, asset_type
- Validate numeric ranges (quantity > 0, percentages 0-100)
- Sanitize user-provided strings before storage

**Authentication/Authorization:**
- Currently designed for single-user local deployment
- Add authentication layer before deploying with network access
- Consider API key protection for public endpoints

**Sensitive Data:**
- No sensitive data (passwords, API keys) should be logged
- Database contains personal financial data - ensure file permissions are restrictive
- iCloud sync exposes data to Apple's infrastructure

### Logging Best Practices

- Use appropriate log levels: INFO for normal operations, WARNING for issues, ERROR for failures
- Include context in log messages: `logger.info(f"Updated price for {symbol}: {price}")`
- Avoid logging sensitive information (account numbers, exact holdings values in production)
- Logs rotate daily and keep 7 days - adjust retention for production needs

## Testing Strategy

### Unit Testing

**Recommended test coverage:**
- Database functions: transaction CRUD, holdings calculation, allocation settings
- Price fetcher: symbol type detection, multi-source fallback, error handling
- Business logic: cost basis calculation, cash linking, allocation percentage calculation

**Test tools:**
- pytest for test framework
- pytest-mock for mocking external dependencies
- sqlite3 in-memory database (`:memory:`) for database tests
- Mock all external API calls (AKShare, Yahoo Finance, etc.)

**Example test structure:**
```python
def test_add_transaction_with_cash_link():
    # Setup: Create in-memory database
    # Execute: Add BUY transaction with link_cash=True
    # Assert: Verify both stock BUY and CASH SELL transactions exist
    # Assert: Verify cash SELL amount = stock total_amount + commission
```

### Integration Testing

**API endpoint testing:**
- Use FastAPI TestClient
- Test all routes in `routers/api.py`
- Verify response formats and status codes
- Test error conditions (404, 422)

**Example:**
```python
from fastapi.testclient import TestClient
client = TestClient(app)
response = client.get("/api/holdings?account_id=test")
assert response.status_code == 200
assert isinstance(response.json(), list)
```

### End-to-End Testing

**User scenario testing:**
- Use Playwright or Selenium for browser automation
- Test complete workflows: add transaction → view holdings → update price → view charts
- Verify Chart.js rendering and data accuracy
- Test form validation and error messages

## File Structure Reference

```
invest-log/
├── app.py                      # Application entry point
├── config.py                   # Configuration management
├── database.py                 # Database operations & business logic
├── price_fetcher.py           # Multi-source price fetching
├── logger_config.py           # Logging configuration
├── requirements.txt           # Runtime dependencies
├── requirements-dev.txt       # Development dependencies
├── invest-log-backend.spec    # PyInstaller build specification
├── package.json               # Electron app configuration
├── package-lock.json          # NPM lockfile
├── electron/                  # Electron main process
│   ├── main.js               # Electron entrypoint
│   └── loading.html          # Loading screen
├── routers/                   # Feature-based routing modules
│   ├── overview.py           # Dashboard & charts
│   ├── holdings.py           # Holdings detail & price updates
│   ├── transactions.py       # Transaction management
│   ├── settings.py           # Configuration & accounts
│   ├── api.py                # REST API endpoints
│   ├── setup.py              # First-run setup wizard
│   └── utils.py              # Template utilities
├── templates/                 # Jinja2 HTML templates
│   ├── base.html             # Base template with navigation
│   ├── index.html            # Portfolio overview
│   ├── holdings.html         # Holdings detail page
│   ├── transactions.html     # Transaction list
│   ├── add.html              # Add transaction form
│   ├── settings.html         # Settings page
│   ├── charts.html           # Charts visualization
│   ├── symbol.html           # Symbol detail page
│   └── setup.html            # Setup wizard
├── static/                    # Static assets
│   └── style.css             # Application styles
└── scripts/
    └── build.sh              # Build automation script
```

## Performance Optimization

### Database Level

**Existing optimizations:**
- Indexes on: symbol, transaction_date, account_id, transaction_type, currency, asset_type
- Row factory for efficient dict-like access
- Pagination (100 items per page) for transaction lists

**Recommendations:**
- Monitor query execution plans for slow queries
- Consider composite indexes for frequent multi-column filters
- Use EXPLAIN QUERY PLAN to analyze complex queries
- Keep statistics updated (ANALYZE command)

**Avoiding performance pitfalls:**
- Don't load entire transaction history at once - use pagination
- Aggregate calculations (holdings, P/L) can be expensive - consider caching results
- Limit concurrent writes to avoid SQLite locking issues

### Application Level

**Price fetching optimization:**
- Price updates are cached in `latest_prices` table
- Implement rate limiting for bulk price updates
- Consider adding configurable cache TTL (time-to-live)
- Batch update multiple symbols with delays to avoid API rate limits

**Template rendering:**
- Keep calculation logic in Python layer, not templates
- Pass pre-computed values to templates
- For large portfolios, consider lazy-loading charts and details

### Network and External Services

**Multi-source fallback strategy:**
- Primary: AKShare (best for A-shares)
- Secondary: Yahoo Finance (reliable for US/HK stocks)
- Tertiary: Sina Finance (backup for A-shares)
- Quaternary: Tencent Finance (additional backup)

**Optimization tips:**
- Set appropriate timeouts for external API calls
- Implement exponential backoff for retries
- Log failed attempts for monitoring
- Consider circuit breaker pattern for unreliable data sources

## Deployment and Operations

### Production Deployment

**Server requirements:**
- Python 3.10+ (3.11 recommended)
- 512MB RAM minimum (1GB+ recommended for production)
- Disk space: ~100MB for application + database size (varies with usage)
- Network access to external price data APIs

**Environment setup:**
```bash
# Install dependencies
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# Set custom data directory (optional)
export INVEST_LOG_DATA_DIR=/path/to/data

# Run with production settings
python app.py --host 0.0.0.0 --port 8000
```

**Process management:**
- Use systemd (Linux) or launchd (macOS) for auto-start
- Configure auto-restart on failure
- Set up log rotation if not using default daily rotation
- Monitor resource usage (CPU, memory, disk)

**Reverse proxy setup (recommended for production):**
```nginx
# Nginx example
location / {
    proxy_pass http://127.0.0.1:8000;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

### Database Backup and Recovery

**Backup strategy:**
- **Frequency**: Daily automated backups, weekly full backups
- **Retention**: Keep 7 daily + 4 weekly backups minimum
- **Method**: File-level copy of SQLite database when application is stopped or during low-activity period
- **Location**: Store backups separate from primary data directory (ideally different disk/server)

**Backup script example:**
```bash
#!/bin/bash
DB_PATH="~/Library/Mobile Documents/com~apple~CloudDocs/InvestLog/transactions.db"
BACKUP_DIR="/path/to/backups"
DATE=$(date +%Y%m%d_%H%M%S)
sqlite3 "$DB_PATH" ".backup '$BACKUP_DIR/transactions_$DATE.db'"
# Keep only last 7 daily backups
find "$BACKUP_DIR" -name "transactions_*.db" -mtime +7 -delete
```

**Recovery procedure:**
1. Stop the application
2. Rename current database (don't delete - keep as emergency fallback)
3. Copy backup to primary location
4. Verify database integrity: `sqlite3 transactions.db "PRAGMA integrity_check;"`
5. Restart application and verify data

### Monitoring and Logging

**Key metrics to monitor:**
- Application uptime and response time
- Database query performance (slow query log)
- Price fetch success rate per data source
- Error rate in logs
- Disk space (database growth, log accumulation)

**Log management:**
- Default: Daily rotation, 7-day retention in `logs/` directory
- Production: Consider centralized logging (ELK stack, Loki, CloudWatch)
- Alert on ERROR level messages
- Monitor WARNING for price fetch failures

**Health check endpoint:**
Consider adding `/health` endpoint that verifies:
- Database connectivity
- Disk space availability
- Last successful price update timestamp

### Version Upgrades

**Before upgrading:**
1. Back up database
2. Test upgrade in staging environment
3. Review changelog for breaking changes
4. Check for database migrations

**Upgrade process:**
1. Stop application
2. Back up current database and code
3. Pull new code version
4. Install updated dependencies: `pip install -r requirements.txt`
5. Run database migrations (if any)
6. Start application and monitor logs
7. Verify critical functions (add transaction, price update, view holdings)
8. If issues occur, rollback to backup

**Database migrations:**
- Current system handles migrations in `init_database()` using try/except for new columns
- For schema changes, prefer additive migrations (add columns, not drop)
- Test migrations with copy of production database before applying

### Container Deployment (Docker)

**Dockerfile considerations:**
```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
ENV INVEST_LOG_DATA_DIR=/data
VOLUME ["/data"]
CMD ["python", "app.py", "--host", "0.0.0.0", "--port", "8000", "--data-dir", "/data"]
```

**Important for containers:**
- Mount `/data` volume for database persistence
- Use environment variables for configuration
- Set appropriate user permissions (don't run as root)
- Expose port 8000
- Include health checks in docker-compose/Kubernetes

## Troubleshooting Guide

### Database Issues

**Problem: Database locked errors**
- Cause: Multiple processes trying to write simultaneously
- Solution: SQLite supports limited concurrent writes. Use single application instance or implement proper write queuing

**Problem: Database corruption**
- Check integrity: `sqlite3 transactions.db "PRAGMA integrity_check;"`
- If corrupt: Restore from backup
- Prevention: Ensure clean shutdowns, avoid killing process during writes

**Problem: Slow queries**
- Use EXPLAIN QUERY PLAN to identify missing indexes
- Check if database needs VACUUM (reclaim unused space)
- Review query patterns and add composite indexes if needed

### Price Fetching Issues

**Problem: All price fetches failing**
- Check internet connectivity
- Verify data source APIs are accessible (test manually)
- Review logs for specific error messages
- Check if API keys/credentials are required (some sources)

**Problem: Specific symbols always fail**
- Verify symbol format matches expected pattern for that exchange
- Check currency matches the symbol's trading currency
- Try updating price manually with different data source
- Some asset types (bonds) don't support auto-fetch

### Application Errors

**Problem: Application won't start**
- Check Python version compatibility (3.10+)
- Verify all dependencies installed: `pip list`
- Check port 8000 not already in use: `lsof -i :8000`
- Review startup logs for specific errors
- Verify data directory permissions

**Problem: Template rendering errors**
- Check template syntax in Jinja2 files
- Verify all required context variables are passed
- Check static files are accessible
- Review browser console for JavaScript errors

## Data Privacy and Security

- All financial data stored locally (SQLite database)
- iCloud sync is optional - disable if privacy concerns exist
- No built-in authentication - add reverse proxy auth for network access
- Price fetching is the only external communication
- Desktop app can run fully offline after initial price cache
- Database file contains sensitive financial information - set restrictive file permissions (600/700)

## Critical Reminders for Development

### Before Making Changes

1. **Always read existing code first** - Don't propose changes without understanding current implementation
2. **Database changes require migration strategy** - Use try/except pattern for additive changes (see init_database())
3. **Test with multiple currencies** - Ensure changes work for CNY, USD, and HKD
4. **Preserve cash linking logic** - Don't break the automatic cash transaction creation
5. **Maintain cost basis accuracy** - Weighted average calculation is critical for tax reporting

### Common Pitfalls to Avoid

- **Don't use string concatenation for SQL** - Always use parameterized queries
- **Don't forget to close database connections** - Memory leaks will occur
- **Don't run multiple write operations concurrently** - SQLite limitation
- **Don't expose internal errors to users** - Log details, show friendly messages
- **Don't skip testing multi-currency scenarios** - Currency mixing causes data corruption
- **Don't modify transaction history without ADJUST type** - Maintain audit trail integrity

### When Adding New Features

- **New transaction type?** Update CHECK constraint in database.py and handle in all calculation functions
- **New currency?** Update CHECK constraint and test all currency-specific logic
- **New asset type?** Use asset_types table, don't hardcode
- **New data source for prices?** Follow existing fallback pattern in price_fetcher.py
- **New route?** Register in app.py, follow existing authentication/validation patterns
- **New database column?** Add migration logic in init_database() with try/except

### Performance Considerations

- Holdings calculation can be expensive for large portfolios - consider caching strategy before optimizing
- Price fetching should respect API rate limits - add delays for bulk updates
- Database queries should use indexes - check execution plan for new query patterns
- Template rendering with large datasets - paginate or lazy-load as needed
