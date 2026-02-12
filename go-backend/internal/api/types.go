package api

type addTransactionPayload struct {
	TransactionDate string   `json:"transaction_date"`
	TransactionTime *string  `json:"transaction_time"`
	Symbol          string   `json:"symbol"`
	TransactionType string   `json:"transaction_type"`
	Quantity        float64  `json:"quantity"`
	Price           float64  `json:"price"`
	AccountID       string   `json:"account_id"`
	AssetType       string   `json:"asset_type"`
	Commission      float64  `json:"commission"`
	Currency        string   `json:"currency"`
	AccountName     *string  `json:"account_name"`
	Notes           *string  `json:"notes"`
	Tags            *string  `json:"tags"`
	TotalAmount     *float64 `json:"total_amount"`
	LinkCash        bool     `json:"link_cash"`
}

type pricePayload struct {
	Symbol    string `json:"symbol"`
	Currency  string `json:"currency"`
	AssetType string `json:"asset_type"`
}

type manualPricePayload struct {
	Symbol   string  `json:"symbol"`
	Currency string  `json:"currency"`
	Price    float64 `json:"price"`
}

type updateAllPricesPayload struct {
	Currency string `json:"currency"`
}

type aiHoldingsAnalysisPayload struct {
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key"`
	Model           string `json:"model"`
	Currency        string `json:"currency"`
	RiskProfile     string `json:"risk_profile"`
	Horizon         string `json:"horizon"`
	AdviceStyle     string `json:"advice_style"`
	AllowNewSymbols *bool  `json:"allow_new_symbols"`
	StrategyPrompt  string `json:"strategy_prompt"`
}

type aiSymbolAnalysisPayload struct {
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	Symbol         string `json:"symbol"`
	Currency       string `json:"currency"`
	RiskProfile    string `json:"risk_profile"`
	Horizon        string `json:"horizon"`
	AdviceStyle    string `json:"advice_style"`
	StrategyPrompt string `json:"strategy_prompt"`
}

type addAccountPayload struct {
	AccountID         string  `json:"account_id"`
	AccountName       string  `json:"account_name"`
	Broker            *string `json:"broker"`
	AccountType       *string `json:"account_type"`
	InitialBalanceCNY float64 `json:"initial_balance_cny"`
	InitialBalanceUSD float64 `json:"initial_balance_usd"`
	InitialBalanceHKD float64 `json:"initial_balance_hkd"`
}

type assetTypePayload struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type allocationPayload struct {
	Currency   string  `json:"currency"`
	AssetType  string  `json:"asset_type"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
}

type exchangeRatePayload struct {
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	Rate         float64 `json:"rate"`
}

type symbolUpdatePayload struct {
	Name       *string `json:"name"`
	AssetType  *string `json:"asset_type"`
	AutoUpdate *int    `json:"auto_update"`
	Sector     *string `json:"sector"`
	Exchange   *string `json:"exchange"`
}

type updateSymbolAssetTypePayload struct {
	AssetType string `json:"asset_type"`
}

type updateSymbolAutoUpdatePayload struct {
	AutoUpdate int `json:"auto_update"`
}

type transferPayload struct {
	TransactionDate string  `json:"transaction_date"`
	Symbol          string  `json:"symbol"`
	Quantity        float64 `json:"quantity"`
	FromAccountID   string  `json:"from_account_id"`
	ToAccountID     string  `json:"to_account_id"`
	FromCurrency    string  `json:"from_currency"`
	ToCurrency      string  `json:"to_currency"`
	Commission      float64 `json:"commission"`
	AssetType       string  `json:"asset_type"`
	Notes           *string `json:"notes"`
}

type storageSwitchPayload struct {
	DBName string `json:"db_name"`
	Create bool   `json:"create"`
}

type storageInfoResponse struct {
	DBName       string   `json:"db_name"`
	DBPath       string   `json:"db_path"`
	DataDir      string   `json:"data_dir"`
	UseICloud    bool     `json:"use_icloud"`
	Available    []string `json:"available"`
	CanSwitch    bool     `json:"can_switch"`
	SwitchReason string   `json:"switch_reason,omitempty"`
}
