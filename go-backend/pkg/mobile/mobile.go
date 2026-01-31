package mobile

import (
	"encoding/json"
	"fmt"

	"investlog/pkg/investlog"
)

// Core wraps the Invest Log core for gomobile bindings.
type Core struct {
	core *investlog.Core
}

// Open initializes the core with a database path.
func Open(dbPath string) (*Core, error) {
	core, err := investlog.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &Core{core: core}, nil
}

// Close releases resources.
func (c *Core) Close() error {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.Close()
}

// GetHoldingsJSON returns holdings as JSON.
func (c *Core) GetHoldingsJSON(accountID string) (string, error) {
	data, err := c.core.GetHoldings(accountID)
	if err != nil {
		return "", err
	}
	return marshalJSON(data)
}

// GetHoldingsByCurrencyJSON returns allocation data as JSON.
func (c *Core) GetHoldingsByCurrencyJSON() (string, error) {
	data, err := c.core.GetHoldingsByCurrency()
	if err != nil {
		return "", err
	}
	return marshalJSON(data)
}

// GetHoldingsBySymbolJSON returns per-symbol holdings as JSON.
func (c *Core) GetHoldingsBySymbolJSON() (string, error) {
	data, err := c.core.GetHoldingsBySymbol()
	if err != nil {
		return "", err
	}
	return marshalJSON(data)
}

// GetTransactionsJSON queries transactions with optional filter JSON.
func (c *Core) GetTransactionsJSON(filterJSON string) (string, error) {
	filter := investlog.TransactionFilter{}
	if filterJSON != "" {
		var payload transactionFilterPayload
		if err := json.Unmarshal([]byte(filterJSON), &payload); err != nil {
			return "", err
		}
		filter = investlog.TransactionFilter{
			Symbol:          payload.Symbol,
			AccountID:       payload.AccountID,
			TransactionType: payload.TransactionType,
			Currency:        payload.Currency,
			Year:            payload.Year,
			StartDate:       payload.StartDate,
			EndDate:         payload.EndDate,
			Limit:           payload.Limit,
			Offset:          payload.Offset,
		}
	}
	data, err := c.core.GetTransactions(filter)
	if err != nil {
		return "", err
	}
	return marshalJSON(data)
}

// AddTransactionJSON creates a transaction from JSON and returns id JSON.
func (c *Core) AddTransactionJSON(payloadJSON string) (string, error) {
	var payload transactionPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return "", err
	}
	id, err := c.core.AddTransaction(investlog.AddTransactionRequest{
		TransactionDate: payload.TransactionDate,
		TransactionTime: payload.TransactionTime,
		Symbol:          payload.Symbol,
		TransactionType: payload.TransactionType,
		Quantity:        payload.Quantity,
		Price:           payload.Price,
		AccountID:       payload.AccountID,
		AssetType:       payload.AssetType,
		Commission:      payload.Commission,
		Currency:        payload.Currency,
		AccountName:     payload.AccountName,
		Notes:           payload.Notes,
		Tags:            payload.Tags,
		TotalAmount:     payload.TotalAmount,
		LinkCash:        payload.LinkCash,
	})
	if err != nil {
		return "", err
	}
	return marshalJSON(map[string]any{"id": id})
}

// DeleteTransaction deletes a transaction by id.
func (c *Core) DeleteTransaction(id int64) (bool, error) {
	return c.core.DeleteTransaction(id)
}

// UpdatePriceJSON fetches and updates price from JSON payload.
func (c *Core) UpdatePriceJSON(payloadJSON string) (string, error) {
	var payload pricePayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return "", err
	}
	result, err := c.core.UpdatePrice(payload.Symbol, payload.Currency)
	if err != nil && result.Price == nil {
		return "", fmt.Errorf(result.Message)
	}
	return marshalJSON(result)
}

// ManualUpdatePrice updates price with provided values.
func (c *Core) ManualUpdatePrice(symbol, currency string, price float64) error {
	return c.core.ManualUpdatePrice(symbol, currency, price)
}

func marshalJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type transactionFilterPayload struct {
	Symbol          string `json:"symbol"`
	AccountID       string `json:"account_id"`
	TransactionType string `json:"transaction_type"`
	Currency        string `json:"currency"`
	Year            int    `json:"year"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	Limit           int    `json:"limit"`
	Offset          int    `json:"offset"`
}

type transactionPayload struct {
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
	Symbol   string `json:"symbol"`
	Currency string `json:"currency"`
}
