package investlog

import (
	"sort"
	"strings"
)

func (c *Core) insertPendingSymbolAnalysis(req SymbolAnalysisRequest) (int64, error) {
	result, err := c.db.Exec(
		`INSERT INTO symbol_analyses (symbol, currency, model, status, strategy_prompt)
		 VALUES (?, ?, ?, 'pending', ?)`,
		req.Symbol, req.Currency, req.Model, req.StrategyPrompt,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (c *Core) updateSymbolAnalysisStatus(id int64, status, errMsg string) error {
	_, err := c.db.Exec(
		`UPDATE symbol_analyses SET status = ?, error_message = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, errMsg, id,
	)
	return err
}

func orderedDimensionOutputKeys(dimensionOutputs map[string]string) []string {
	orderedKeys := make([]string, 0, len(dimensionOutputs))
	seen := make(map[string]struct{}, len(dimensionOutputs))

	for _, framework := range symbolFrameworkCatalog {
		output := strings.TrimSpace(dimensionOutputs[framework.ID])
		if output == "" {
			continue
		}
		orderedKeys = append(orderedKeys, framework.ID)
		seen[framework.ID] = struct{}{}
	}

	for _, legacyKey := range legacyDimensionColumnOrder {
		output := strings.TrimSpace(dimensionOutputs[legacyKey])
		if output == "" {
			continue
		}
		if _, exists := seen[legacyKey]; exists {
			continue
		}
		orderedKeys = append(orderedKeys, legacyKey)
		seen[legacyKey] = struct{}{}
	}

	var extras []string
	for key, output := range dimensionOutputs {
		if strings.TrimSpace(output) == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		extras = append(extras, key)
	}
	sort.Strings(extras)
	orderedKeys = append(orderedKeys, extras...)
	return orderedKeys
}

func mapDimensionOutputsToLegacyColumns(dimensionOutputs map[string]string) (string, string, string, string) {
	var slots [4]string
	orderedKeys := orderedDimensionOutputKeys(dimensionOutputs)
	for idx, key := range orderedKeys {
		if idx >= len(slots) {
			break
		}
		slots[idx] = dimensionOutputs[key]
	}
	return slots[0], slots[1], slots[2], slots[3]
}

func orderedDimensionIDs(dimensions map[string]*SymbolDimensionResult) []string {
	if len(dimensions) == 0 {
		return nil
	}

	ordered := make([]string, 0, len(dimensions))
	seen := make(map[string]struct{}, len(dimensions))
	for _, framework := range symbolFrameworkCatalog {
		if _, ok := dimensions[framework.ID]; !ok {
			continue
		}
		ordered = append(ordered, framework.ID)
		seen[framework.ID] = struct{}{}
	}
	for _, legacyKey := range legacyDimensionColumnOrder {
		if _, ok := dimensions[legacyKey]; !ok {
			continue
		}
		if _, exists := seen[legacyKey]; exists {
			continue
		}
		ordered = append(ordered, legacyKey)
		seen[legacyKey] = struct{}{}
	}

	var extras []string
	for key := range dimensions {
		if _, exists := seen[key]; exists {
			continue
		}
		extras = append(extras, key)
	}
	sort.Strings(extras)
	ordered = append(ordered, extras...)
	return ordered
}

func (c *Core) saveCompletedSymbolAnalysis(id int64, dimensionOutputs map[string]string, synthesisOutput string, externalDataSummary string) error {
	macroOutput, industryOutput, companyOutput, internationalOutput := mapDimensionOutputsToLegacyColumns(dimensionOutputs)

	_, err := c.db.Exec(
		`UPDATE symbol_analyses
		 SET status = 'completed',
		     macro_analysis = ?,
		     industry_analysis = ?,
		     company_analysis = ?,
		     international_analysis = ?,
		     synthesis = ?,
		     external_data_summary = ?,
		     completed_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		macroOutput,
		industryOutput,
		companyOutput,
		internationalOutput,
		synthesisOutput,
		externalDataSummary,
		id,
	)
	return err
}
