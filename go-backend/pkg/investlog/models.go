package investlog

var Currencies = []string{"CNY", "USD", "HKD"}

var DefaultAssetTypes = []string{"stock", "bond", "metal", "cash"}

var DefaultAssetTypeLabels = map[string]string{
	"stock": "股票",
	"bond":  "债券",
	"metal": "贵金属",
	"cash":  "现金",
}

var TransactionTypes = []string{
	"BUY",
	"SELL",
	"DIVIDEND",
	"SPLIT",
	"TRANSFER_IN",
	"TRANSFER_OUT",
	"ADJUST",
	"INCOME",
}

// Transaction represents a transaction record with symbol metadata.
type Transaction struct {
	ID                  int64   `json:"id"`
	TransactionDate     string  `json:"transaction_date"`
	TransactionTime     *string `json:"transaction_time"`
	SymbolID            int64   `json:"symbol_id"`
	Symbol              string  `json:"symbol"`
	Name                *string `json:"name"`
	AssetType           string  `json:"asset_type"`
	TransactionType     string  `json:"transaction_type"`
	Quantity            Amount  `json:"quantity"`
	Price               Amount  `json:"price"`
	TotalAmount         Amount  `json:"total_amount"`
	Commission          Amount  `json:"commission"`
	Currency            string  `json:"currency"`
	AccountID           string  `json:"account_id"`
	AccountName         *string `json:"account_name"`
	Notes               *string `json:"notes"`
	Tags                *string `json:"tags"`
	LinkedTransactionID *int64  `json:"linked_transaction_id"`
	CreatedAt           *string `json:"created_at"`
	UpdatedAt           *string `json:"updated_at"`
}

// AddTransactionRequest defines inputs to add a transaction.
type AddTransactionRequest struct {
	TransactionDate string
	TransactionTime *string
	Symbol          string
	TransactionType string
	Quantity        Amount
	Price           Amount
	AccountID       string
	AssetType       string
	Commission      Amount
	Currency        string
	AccountName     *string
	Notes           *string
	Tags            *string
	TotalAmount     *Amount
	LinkCash        bool
}

// TransferRequest defines inputs for a cross-account transfer.
type TransferRequest struct {
	TransactionDate string
	Symbol          string
	Quantity        Amount
	FromAccountID   string
	ToAccountID     string
	FromCurrency    string
	ToCurrency      string
	Commission      Amount
	AssetType       string
	Notes           *string
}

// TransferResult returns the IDs of the paired transactions.
type TransferResult struct {
	TransferOutID int64  `json:"transfer_out_id"`
	TransferInID  int64  `json:"transfer_in_id"`
	ExchangeRate  Amount `json:"exchange_rate,omitempty"`
}

// Holding represents a current holding snapshot.
type Holding struct {
	Symbol      string  `json:"symbol"`
	Name        *string `json:"name"`
	AccountID   string  `json:"account_id"`
	Currency    string  `json:"currency"`
	AssetType   string  `json:"asset_type"`
	TotalShares Amount  `json:"total_shares"`
	TotalCost   Amount  `json:"total_cost"`
	AvgCost     Amount  `json:"avg_cost"`
}

// AllocationEntry represents allocation summary per asset type.
type AllocationEntry struct {
	AssetType  string  `json:"asset_type"`
	Label      string  `json:"label"`
	Amount     Amount  `json:"amount"`
	Percent    float64 `json:"percent"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
	Warning    *string `json:"warning"`
}

// CurrencyAllocation holds allocation data for a currency.
type CurrencyAllocation struct {
	Total       Amount            `json:"total"`
	Allocations []AllocationEntry `json:"allocations"`
}

// HoldingsByCurrencyResult maps currency to allocation data.
type HoldingsByCurrencyResult map[string]CurrencyAllocation

// SymbolHolding represents per-symbol holding details.
type SymbolHolding struct {
	Symbol         string   `json:"symbol"`
	Name           *string  `json:"name"`
	DisplayName    string   `json:"display_name"`
	AssetType      string   `json:"asset_type"`
	AssetTypeLabel string   `json:"asset_type_label"`
	AutoUpdate     int      `json:"auto_update"`
	AccountID      string   `json:"account_id"`
	AccountName    string   `json:"account_name"`
	TotalShares    Amount   `json:"total_shares"`
	AvgCost        Amount   `json:"avg_cost"`
	CostBasis      Amount   `json:"cost_basis"`
	LatestPrice    *Amount  `json:"latest_price"`
	PriceUpdatedAt *string  `json:"price_updated_at"`
	MarketValue    Amount   `json:"market_value"`
	UnrealizedPnL  *Amount  `json:"unrealized_pnl"`
	PnlPercent     *float64 `json:"pnl_percent"`
	Percent        float64  `json:"percent"`
}

// SymbolHoldingsByAccount groups symbols by account for chart legend.
type SymbolHoldingsByAccount struct {
	AccountName string          `json:"account_name"`
	Symbols     []SymbolHolding `json:"symbols"`
}

// SymbolHoldingsCurrency aggregates symbol holdings for a currency.
type SymbolHoldingsCurrency struct {
	TotalCost        Amount                             `json:"total_cost"`
	TotalMarketValue Amount                             `json:"total_market_value"`
	TotalPnL         Amount                             `json:"total_pnl"`
	Symbols          []SymbolHolding                    `json:"symbols"`
	ByAccount        map[string]SymbolHoldingsByAccount `json:"by_account"`
}

// HoldingsBySymbolResult maps currency to symbol holdings.
type HoldingsBySymbolResult map[string]SymbolHoldingsCurrency

// AccountSymbolHolding represents per-symbol holdings within an account.
type AccountSymbolHolding struct {
	Symbol         string  `json:"symbol"`
	Name           *string `json:"name"`
	DisplayName    string  `json:"display_name"`
	AssetType      string  `json:"asset_type"`
	AssetTypeLabel string  `json:"asset_type_label"`
	MarketValue    Amount  `json:"market_value"`
	TotalShares    Amount  `json:"total_shares"`
	Percent        float64 `json:"percent"`
}

// AccountHoldings aggregates holdings for a single account.
type AccountHoldings struct {
	AccountName      string                 `json:"account_name"`
	TotalMarketValue Amount                 `json:"total_market_value"`
	Symbols          []AccountSymbolHolding `json:"symbols"`
}

// CurrencyAccountHoldings aggregates account holdings for a currency.
type CurrencyAccountHoldings struct {
	TotalMarketValue Amount                     `json:"total_market_value"`
	Accounts         map[string]AccountHoldings `json:"accounts"`
}

// HoldingsByCurrencyAccountResult maps currency to account holdings.
type HoldingsByCurrencyAccountResult map[string]CurrencyAccountHoldings

// AllocationSetting represents allocation thresholds.
type AllocationSetting struct {
	ID         int64   `json:"id"`
	Currency   string  `json:"currency"`
	AssetType  string  `json:"asset_type"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
}

// ExchangeRateSetting represents a maintained FX rate.
type ExchangeRateSetting struct {
	ID           int64  `json:"id"`
	FromCurrency string `json:"from_currency"`
	ToCurrency   string `json:"to_currency"`
	Rate         Amount `json:"rate"`
	Source       string `json:"source"`
	UpdatedAt    string `json:"updated_at"`
}

// AISettings represents persisted AI analysis configuration (excluding API key).
type AISettings struct {
	BaseURL         string `json:"base_url"`
	Model           string `json:"model"`
	RiskProfile     string `json:"risk_profile"`
	Horizon         string `json:"horizon"`
	AdviceStyle     string `json:"advice_style"`
	AllowNewSymbols bool   `json:"allow_new_symbols"`
	StrategyPrompt  string `json:"strategy_prompt"`
}

// AssetType represents a dynamic asset type.
type AssetType struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Label     string `json:"label"`
	CreatedAt string `json:"created_at"`
}

// Account represents an investment account.
type Account struct {
	AccountID   string  `json:"account_id"`
	AccountName string  `json:"account_name"`
	Broker      *string `json:"broker"`
	AccountType *string `json:"account_type"`
	CreatedAt   *string `json:"created_at"`
}

// Symbol represents symbol metadata.
type Symbol struct {
	ID         int64   `json:"id"`
	Symbol     string  `json:"symbol"`
	Name       *string `json:"name"`
	AssetType  string  `json:"asset_type"`
	Sector     *string `json:"sector"`
	Exchange   *string `json:"exchange"`
	AutoUpdate int     `json:"auto_update"`
}

// LatestPrice represents the last fetched price for a symbol.
type LatestPrice struct {
	Symbol    string `json:"symbol"`
	Currency  string `json:"currency"`
	Price     Amount `json:"price"`
	UpdatedAt string `json:"updated_at"`
}

// OperationLog represents an audit log record.
type OperationLog struct {
	ID           int64   `json:"id"`
	Operation    string  `json:"operation_type"`
	Symbol       *string `json:"symbol"`
	Currency     *string `json:"currency"`
	Details      *string `json:"details"`
	OldValue     *Amount `json:"old_value"`
	NewValue     *Amount `json:"new_value"`
	PriceFetched *Amount `json:"price_fetched"`
	CreatedAt    *string `json:"created_at"`
}

// PortfolioPoint represents a cumulative portfolio history point.
type PortfolioPoint struct {
	Date  string `json:"date"`
	Value Amount `json:"value"`
}

// PriceResult represents a fetch price result.
type PriceResult struct {
	Price   *Amount `json:"price"`
	Message string  `json:"message"`
}

// Time helpers.
func todayISO() string {
	return TodayISOInShanghai()
}
