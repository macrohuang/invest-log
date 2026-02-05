package investlog

import (
	"fmt"
	"sync"
	"time"
)

// UpdatePrice fetches and stores latest price for a symbol.
func (c *Core) UpdatePrice(symbol, currency, assetType string) (PriceResult, error) {
	result, err := c.FetchPrice(symbol, currency, assetType)
	if result.Price != nil {
		_ = c.UpdateLatestPrice(symbol, currency, *result.Price)
		_, _ = c.AddOperationLog(OperationLog{
			Operation:    "PRICE_UPDATE",
			Symbol:       stringPtr(normalizeSymbol(symbol)),
			Currency:     stringPtr(normalizeCurrency(currency)),
			Details:      stringPtr(result.Message),
			PriceFetched: result.Price,
		})
		return result, nil
	}
	_, _ = c.AddOperationLog(OperationLog{
		Operation: "PRICE_UPDATE_FAILED",
		Symbol:    stringPtr(normalizeSymbol(symbol)),
		Currency:  stringPtr(normalizeCurrency(currency)),
		Details:   stringPtr(result.Message),
	})
	return result, err
}

// ManualUpdatePrice stores a manual price override.
func (c *Core) ManualUpdatePrice(symbol, currency string, price float64) error {
	if err := c.UpdateLatestPrice(symbol, currency, price); err != nil {
		return err
	}
	_, _ = c.AddOperationLog(OperationLog{
		Operation:    "MANUAL_PRICE_UPDATE",
		Symbol:       stringPtr(normalizeSymbol(symbol)),
		Currency:     stringPtr(normalizeCurrency(currency)),
		Details:      stringPtr("Manual price update"),
		PriceFetched: floatPtr(price),
	})
	return nil
}

// UpdateAllPrices updates all auto-update symbols within a currency.
func (c *Core) UpdateAllPrices(currency string) (int, []string, error) {
	currency = normalizeCurrency(currency)
	holdings, err := c.GetHoldingsBySymbol()
	if err != nil {
		return 0, nil, err
	}
	currencyData, ok := holdings[currency]
	if !ok {
		return 0, nil, fmt.Errorf("currency not found")
	}

	const recentThreshold = 5 * time.Minute
	type symbolJob struct {
		symbol    string
		assetType string
	}
	jobs := make([]symbolJob, 0, len(currencyData.Symbols))
	for _, s := range currencyData.Symbols {
		if s.AutoUpdate == 0 {
			continue
		}
		if recentlyUpdated(s.PriceUpdatedAt, recentThreshold) {
			continue
		}
		jobs = append(jobs, symbolJob{symbol: s.Symbol, assetType: s.AssetType})
	}
	if len(jobs) == 0 {
		return 0, nil, nil
	}

	workerCount := updateWorkerCount(len(jobs))
	jobsCh := make(chan symbolJob)
	resultsCh := make(chan updateResult, len(jobs))
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsCh {
				result, err := c.UpdatePrice(job.symbol, currency, job.assetType)
				resultsCh <- updateResult{
					symbol:  job.symbol,
					message: result.Message,
					updated: result.Price != nil,
					err:     err,
				}
			}
		}()
	}

	go func() {
		for _, job := range jobs {
			jobsCh <- job
		}
		close(jobsCh)
		wg.Wait()
		close(resultsCh)
	}()

	updated := 0
	var errors []string
	for res := range resultsCh {
		if res.updated {
			updated++
			continue
		}
		if res.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", res.symbol, res.message))
		}
	}
	return updated, errors, nil
}

type updateResult struct {
	symbol  string
	message string
	updated bool
	err     error
}

func updateWorkerCount(total int) int {
	if total <= 0 {
		return 0
	}
	if total < 4 {
		return total
	}
	return 4
}

func recentlyUpdated(updatedAt *string, threshold time.Duration) bool {
	if updatedAt == nil || *updatedAt == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02 15:04:05", *updatedAt)
	if err != nil {
		return false
	}
	return time.Since(parsed) < threshold
}
