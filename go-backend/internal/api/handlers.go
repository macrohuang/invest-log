package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"investlog/pkg/investlog"
)

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) getHoldings(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	result, err := h.core.GetHoldings(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getHoldingsByCurrency(w http.ResponseWriter, r *http.Request) {
	result, err := h.core.GetHoldingsByCurrency()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getHoldingsBySymbol(w http.ResponseWriter, r *http.Request) {
	result, err := h.core.GetHoldingsBySymbol()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getHoldingsByCurrencyAndAccount(w http.ResponseWriter, r *http.Request) {
	result, err := h.core.GetHoldingsByCurrencyAndAccount()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getTransactions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := investlog.TransactionFilter{
		Symbol:          query.Get("symbol"),
		AccountID:       query.Get("account_id"),
		TransactionType: query.Get("transaction_type"),
		Currency:        query.Get("currency"),
		Year:            parseInt(query.Get("year")),
		StartDate:       query.Get("start_date"),
		EndDate:         query.Get("end_date"),
		Limit:           parseIntDefault(query.Get("limit"), 100),
		Offset:          parseIntDefault(query.Get("offset"), 0),
	}
	limit, offset := normalizeLimitOffset(filter.Limit, filter.Offset)
	filter.Limit = limit
	filter.Offset = offset
	result, err := h.core.GetTransactions(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if query.Get("paged") != "1" {
		writeJSON(w, http.StatusOK, result)
		return
	}
	total, err := h.core.GetTransactionCount(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, transactionsResponse{
		Items:  result,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *handler) addTransaction(w http.ResponseWriter, r *http.Request) {
	var payload addTransactionPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := h.core.AddTransaction(investlog.AddTransactionRequest{
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *handler) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	deleted, err := h.core.DeleteTransaction(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *handler) addTransfer(w http.ResponseWriter, r *http.Request) {
	var payload transferPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.core.Transfer(investlog.TransferRequest{
		TransactionDate: payload.TransactionDate,
		Symbol:          payload.Symbol,
		Quantity:        payload.Quantity,
		FromAccountID:   payload.FromAccountID,
		ToAccountID:     payload.ToAccountID,
		FromCurrency:    payload.FromCurrency,
		ToCurrency:      payload.ToCurrency,
		Commission:      payload.Commission,
		AssetType:       payload.AssetType,
		Notes:           payload.Notes,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getPortfolioHistory(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 1000)
	result, err := h.core.GetPortfolioHistory(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) updatePrice(w http.ResponseWriter, r *http.Request) {
	var payload pricePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.core.UpdatePrice(payload.Symbol, payload.Currency, payload.AssetType)
	if err != nil && result.Price == nil {
		writeError(w, http.StatusBadRequest, result.Message)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) manualUpdatePrice(w http.ResponseWriter, r *http.Request) {
	var payload manualPricePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.core.ManualUpdatePrice(payload.Symbol, payload.Currency, payload.Price); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *handler) updateAllPrices(w http.ResponseWriter, r *http.Request) {
	var payload updateAllPricesPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	count, errors, err := h.core.UpdateAllPrices(payload.Currency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": count, "errors": errors})
}

func (h *handler) analyzeHoldingsWithAI(w http.ResponseWriter, r *http.Request) {
	var payload aiHoldingsAnalysisPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	allowNewSymbols := true
	if payload.AllowNewSymbols != nil {
		allowNewSymbols = *payload.AllowNewSymbols
	}

	result, err := h.core.AnalyzeHoldings(investlog.HoldingsAnalysisRequest{
		BaseURL:         payload.BaseURL,
		APIKey:          payload.APIKey,
		Model:           payload.Model,
		Currency:        payload.Currency,
		RiskProfile:     payload.RiskProfile,
		Horizon:         payload.Horizon,
		AdviceStyle:     payload.AdviceStyle,
		AllowNewSymbols: allowNewSymbols,
		StrategyPrompt:  payload.StrategyPrompt,
	})
	if err != nil {
		h.logger.Error("ai holdings analysis failed",
			"currency", payload.Currency,
			"model", payload.Model,
			"base_url", payload.BaseURL,
			"err", err,
		)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) analyzeSymbolWithAI(w http.ResponseWriter, r *http.Request) {
	var payload aiSymbolAnalysisPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.core.AnalyzeSymbol(investlog.SymbolAnalysisRequest{
		BaseURL:        payload.BaseURL,
		APIKey:         payload.APIKey,
		Model:          payload.Model,
		Symbol:         payload.Symbol,
		Currency:       payload.Currency,
		StrategyPrompt: payload.StrategyPrompt,
	})
	if err != nil {
		h.logger.Error("ai symbol analysis failed",
			"symbol", payload.Symbol,
			"currency", payload.Currency,
			"model", payload.Model,
			"err", err,
		)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getSymbolAnalysis(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	currency := r.URL.Query().Get("currency")
	if symbol == "" || currency == "" {
		writeError(w, http.StatusBadRequest, "symbol and currency are required")
		return
	}
	result, err := h.core.GetSymbolAnalysis(symbol, currency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) getSymbolAnalysisHistory(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	currency := r.URL.Query().Get("currency")
	if symbol == "" || currency == "" {
		writeError(w, http.StatusBadRequest, "symbol and currency are required")
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	results, err := h.core.GetSymbolAnalysisHistory(symbol, currency, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *handler) getAccounts(w http.ResponseWriter, r *http.Request) {
	result, err := h.core.GetAccounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) addAccount(w http.ResponseWriter, r *http.Request) {
	var payload addAccountPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	success, err := h.core.AddAccount(investlog.Account{
		AccountID:   payload.AccountID,
		AccountName: payload.AccountName,
		Broker:      payload.Broker,
		AccountType: payload.AccountType,
	})
	if err != nil || !success {
		writeError(w, http.StatusBadRequest, "add account failed")
		return
	}

	balances := map[string]float64{
		"CNY": payload.InitialBalanceCNY,
		"USD": payload.InitialBalanceUSD,
		"HKD": payload.InitialBalanceHKD,
	}
	for currency, amount := range balances {
		if amount > 0 {
			_, _ = h.core.AddTransaction(investlog.AddTransactionRequest{
				TransactionDate: time.Now().Format("2006-01-02"),
				Symbol:          "CASH",
				TransactionType: "TRANSFER_IN",
				AssetType:       "cash",
				Quantity:        amount,
				Price:           1.0,
				AccountID:       payload.AccountID,
				Currency:        currency,
				Notes:           ptrString("Initial balance"),
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "created"})
}

func (h *handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	deleted, message, err := h.core.DeleteAccount(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": message})
}

func (h *handler) getAssetTypes(w http.ResponseWriter, r *http.Request) {
	result, err := h.core.GetAssetTypes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) addAssetType(w http.ResponseWriter, r *http.Request) {
	var payload assetTypePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err := h.core.AddAssetType(payload.Code, payload.Label)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "created"})
}

func (h *handler) deleteAssetType(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	deleted, message, err := h.core.DeleteAssetType(code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": message})
}

func (h *handler) getAllocationSettings(w http.ResponseWriter, r *http.Request) {
	currency := r.URL.Query().Get("currency")
	result, err := h.core.GetAllocationSettings(currency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) setAllocationSetting(w http.ResponseWriter, r *http.Request) {
	var payload allocationPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err := h.core.SetAllocationSetting(payload.Currency, payload.AssetType, payload.MinPercent, payload.MaxPercent)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *handler) deleteAllocationSetting(w http.ResponseWriter, r *http.Request) {
	var payload allocationPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	deleted, err := h.core.DeleteAllocationSetting(payload.Currency, payload.AssetType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "allocation setting not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *handler) getExchangeRates(w http.ResponseWriter, r *http.Request) {
	result, err := h.core.GetExchangeRates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) setExchangeRate(w http.ResponseWriter, r *http.Request) {
	var payload exchangeRatePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err := h.core.SetExchangeRate(payload.FromCurrency, payload.ToCurrency, payload.Rate, "manual")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *handler) refreshExchangeRates(w http.ResponseWriter, r *http.Request) {
	updated, errors, err := h.core.RefreshExchangeRates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"updated": updated,
		"errors":  errors,
	})
}

func (h *handler) getSymbols(w http.ResponseWriter, r *http.Request) {
	result, err := h.core.GetSymbols()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) updateSymbol(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	var payload symbolUpdatePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.core.UpdateSymbolMetadata(symbol, payload.Name, payload.AssetType, payload.AutoUpdate, payload.Sector, payload.Exchange)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !updated {
		writeError(w, http.StatusNotFound, "symbol not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *handler) updateSymbolAssetType(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	var payload updateSymbolAssetTypePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, oldType, newType, err := h.core.UpdateSymbolAssetType(symbol, payload.AssetType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !updated {
		writeError(w, http.StatusNotFound, "symbol not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"old_type": oldType, "new_type": newType})
}

func (h *handler) updateSymbolAutoUpdate(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	var payload updateSymbolAutoUpdatePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err := h.core.UpdateSymbolAutoUpdate(symbol, payload.AutoUpdate)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *handler) getOperationLogs(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	result, err := h.core.GetOperationLogs(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Helpers.

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func parseInt(value string) int {
	if value == "" {
		return 0
	}
	i, _ := strconv.Atoi(value)
	return i
}

func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return i
}

func normalizeLimitOffset(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

type transactionsResponse struct {
	Items  []investlog.Transaction `json:"items"`
	Total  int                     `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}

func ptrString(value string) *string {
	return &value
}
