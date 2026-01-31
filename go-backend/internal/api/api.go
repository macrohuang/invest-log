package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"investlog/pkg/investlog"
)

// NewRouter builds the HTTP API router.
func NewRouter(core *investlog.Core) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	h := &handler{core: core}

	r.Get("/api/health", h.health)
	// Holdings
	r.Get("/api/holdings", h.getHoldings)
	r.Get("/api/holdings-by-currency", h.getHoldingsByCurrency)
	r.Get("/api/holdings-by-symbol", h.getHoldingsBySymbol)
	r.Get("/api/holdings-by-currency-account", h.getHoldingsByCurrencyAndAccount)

	// Transactions
	r.Get("/api/transactions", h.getTransactions)
	r.Post("/api/transactions", h.addTransaction)
	r.Delete("/api/transactions/{id}", h.deleteTransaction)

	// Portfolio history
	r.Get("/api/portfolio-history", h.getPortfolioHistory)

	// Prices
	r.Post("/api/prices/update", h.updatePrice)
	r.Post("/api/prices/manual", h.manualUpdatePrice)
	r.Post("/api/prices/update-all", h.updateAllPrices)

	// Accounts
	r.Get("/api/accounts", h.getAccounts)
	r.Post("/api/accounts", h.addAccount)
	r.Delete("/api/accounts/{id}", h.deleteAccount)

	// Asset types
	r.Get("/api/asset-types", h.getAssetTypes)
	r.Post("/api/asset-types", h.addAssetType)
	r.Delete("/api/asset-types/{code}", h.deleteAssetType)

	// Allocation settings
	r.Get("/api/allocation-settings", h.getAllocationSettings)
	r.Put("/api/allocation-settings", h.setAllocationSetting)
	r.Delete("/api/allocation-settings", h.deleteAllocationSetting)

	// Symbols
	r.Get("/api/symbols", h.getSymbols)
	r.Put("/api/symbols/{symbol}", h.updateSymbol)
	r.Post("/api/symbols/{symbol}/asset-type", h.updateSymbolAssetType)
	r.Post("/api/symbols/{symbol}/auto-update", h.updateSymbolAutoUpdate)

	// Operation logs
	r.Get("/api/operation-logs", h.getOperationLogs)

	return r
}

type handler struct {
	core *investlog.Core
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
