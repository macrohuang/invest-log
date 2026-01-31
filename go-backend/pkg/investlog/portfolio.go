package investlog

import "sort"

// GetPortfolioHistory returns cumulative BUY/SELL cash flow over time.
func (c *Core) GetPortfolioHistory(limit int) ([]PortfolioPoint, error) {
	if limit <= 0 {
		limit = 1000
	}
	transactions, err := c.GetTransactions(TransactionFilter{Limit: limit})
	if err != nil {
		return nil, err
	}

	byDate := map[string]float64{}
	for _, t := range transactions {
		if t.TransactionType != "BUY" && t.TransactionType != "SELL" {
			continue
		}
		date := t.TransactionDate
		if date == "" {
			continue
		}
		if t.TransactionType == "BUY" {
			byDate[date] += t.TotalAmount
		} else if t.TransactionType == "SELL" {
			byDate[date] -= t.TotalAmount
		}
	}

	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	var cumulative []PortfolioPoint
	var running float64
	for _, d := range dates {
		running += byDate[d]
		cumulative = append(cumulative, PortfolioPoint{Date: d, Value: running})
	}
	return cumulative, nil
}
