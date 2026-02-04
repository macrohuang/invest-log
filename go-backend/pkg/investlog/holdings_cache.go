package investlog

import "sync"

type holdingsCache struct {
	mu                  sync.RWMutex
	holdings            []Holding
	bySymbol            HoldingsBySymbolResult
	byCurrency          HoldingsByCurrencyResult
	byCurrencyAccount   HoldingsByCurrencyAccountResult
	holdingsValid       bool
	bySymbolValid       bool
	byCurrencyValid     bool
	byCurrencyAcctValid bool
}

func newHoldingsCache() *holdingsCache {
	return &holdingsCache{}
}

func (c *holdingsCache) getHoldings() ([]Holding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.holdingsValid {
		return nil, false
	}
	copied := append([]Holding(nil), c.holdings...)
	return copied, true
}

func (c *holdingsCache) setHoldings(items []Holding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.holdings = append([]Holding(nil), items...)
	c.holdingsValid = true
}

func (c *holdingsCache) getBySymbol() (HoldingsBySymbolResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.bySymbolValid {
		return nil, false
	}
	return c.bySymbol, true
}

func (c *holdingsCache) setBySymbol(result HoldingsBySymbolResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bySymbol = result
	c.bySymbolValid = true
}

func (c *holdingsCache) getByCurrency() (HoldingsByCurrencyResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.byCurrencyValid {
		return nil, false
	}
	return c.byCurrency, true
}

func (c *holdingsCache) setByCurrency(result HoldingsByCurrencyResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byCurrency = result
	c.byCurrencyValid = true
}

func (c *holdingsCache) getByCurrencyAccount() (HoldingsByCurrencyAccountResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.byCurrencyAcctValid {
		return nil, false
	}
	return c.byCurrencyAccount, true
}

func (c *holdingsCache) setByCurrencyAccount(result HoldingsByCurrencyAccountResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byCurrencyAccount = result
	c.byCurrencyAcctValid = true
}

func (c *holdingsCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.holdings = nil
	c.bySymbol = nil
	c.byCurrency = nil
	c.byCurrencyAccount = nil
	c.holdingsValid = false
	c.bySymbolValid = false
	c.byCurrencyValid = false
	c.byCurrencyAcctValid = false
}
