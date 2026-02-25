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
	Quantity            float64 `json:"quantity"`
	Price               float64 `json:"price"`
	TotalAmount         float64 `json:"total_amount"`
	Commission          float64 `json:"commission"`
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
	Quantity        float64
	Price           float64
	AccountID       string
	AssetType       string
	Commission      float64
	Currency        string
	AccountName     *string
	Notes           *string
	Tags            *string
	TotalAmount     *float64
	LinkCash        bool
}

// TransferRequest defines inputs for a cross-account transfer.
type TransferRequest struct {
	TransactionDate string
	Symbol          string
	Quantity        float64
	FromAccountID   string
	ToAccountID     string
	FromCurrency    string
	ToCurrency      string
	Commission      float64
	AssetType       string
	Notes           *string
}

// TransferResult returns the IDs of the paired transactions.
type TransferResult struct {
	TransferOutID int64   `json:"transfer_out_id"`
	TransferInID  int64   `json:"transfer_in_id"`
	ExchangeRate  float64 `json:"exchange_rate,omitempty"`
}

// Holding represents a current holding snapshot.
type Holding struct {
	Symbol      string  `json:"symbol"`
	Name        *string `json:"name"`
	AccountID   string  `json:"account_id"`
	Currency    string  `json:"currency"`
	AssetType   string  `json:"asset_type"`
	TotalShares float64 `json:"total_shares"`
	TotalCost   float64 `json:"total_cost"`
	AvgCost     float64 `json:"avg_cost"`
}

// AllocationEntry represents allocation summary per asset type.
type AllocationEntry struct {
	AssetType  string  `json:"asset_type"`
	Label      string  `json:"label"`
	Amount     float64 `json:"amount"`
	Percent    float64 `json:"percent"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
	Warning    *string `json:"warning"`
}

// CurrencyAllocation holds allocation data for a currency.
type CurrencyAllocation struct {
	Total       float64           `json:"total"`
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
	TotalShares    float64  `json:"total_shares"`
	AvgCost        float64  `json:"avg_cost"`
	CostBasis      float64  `json:"cost_basis"`
	LatestPrice    *float64 `json:"latest_price"`
	PriceUpdatedAt *string  `json:"price_updated_at"`
	MarketValue    float64  `json:"market_value"`
	UnrealizedPnL  *float64 `json:"unrealized_pnl"`
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
	TotalCost        float64                            `json:"total_cost"`
	TotalMarketValue float64                            `json:"total_market_value"`
	TotalPnL         float64                            `json:"total_pnl"`
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
	MarketValue    float64 `json:"market_value"`
	TotalShares    float64 `json:"total_shares"`
	Percent        float64 `json:"percent"`
}

// AccountHoldings aggregates holdings for a single account.
type AccountHoldings struct {
	AccountName      string                 `json:"account_name"`
	TotalMarketValue float64                `json:"total_market_value"`
	Symbols          []AccountSymbolHolding `json:"symbols"`
}

// CurrencyAccountHoldings aggregates account holdings for a currency.
type CurrencyAccountHoldings struct {
	TotalMarketValue float64                    `json:"total_market_value"`
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
	ID           int64   `json:"id"`
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	Rate         float64 `json:"rate"`
	Source       string  `json:"source"`
	UpdatedAt    string  `json:"updated_at"`
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
	Symbol    string  `json:"symbol"`
	Currency  string  `json:"currency"`
	Price     float64 `json:"price"`
	UpdatedAt string  `json:"updated_at"`
}

// OperationLog represents an audit log record.
type OperationLog struct {
	ID           int64    `json:"id"`
	Operation    string   `json:"operation_type"`
	Symbol       *string  `json:"symbol"`
	Currency     *string  `json:"currency"`
	Details      *string  `json:"details"`
	OldValue     *float64 `json:"old_value"`
	NewValue     *float64 `json:"new_value"`
	PriceFetched *float64 `json:"price_fetched"`
	CreatedAt    *string  `json:"created_at"`
}

// PortfolioPoint represents a cumulative portfolio history point.
type PortfolioPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// PriceResult represents a fetch price result.
type PriceResult struct {
	Price   *float64 `json:"price"`
	Message string   `json:"message"`
}

// Time helpers.
func todayISO() string {
	return TodayISOInShanghai()
}
