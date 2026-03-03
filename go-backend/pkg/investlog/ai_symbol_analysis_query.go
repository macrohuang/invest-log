package investlog

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetSymbolAnalysis returns the latest completed analysis for a symbol.
func (c *Core) GetSymbolAnalysis(symbol, currency string) (*SymbolAnalysisResult, error) {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	currency = strings.TrimSpace(strings.ToUpper(currency))

	var (
		id               int64
		model, status    string
		macroRaw         sql.NullString
		industryRaw      sql.NullString
		companyRaw       sql.NullString
		internationalRaw sql.NullString
		synthesisRaw     sql.NullString
		errorMessage     sql.NullString
		createdAt        string
		completedAtRaw   sql.NullString
	)

	err := c.db.QueryRow(
		`SELECT id, model, status, macro_analysis, industry_analysis, company_analysis, international_analysis,
		        synthesis, error_message, created_at, completed_at
		 FROM symbol_analyses
		 WHERE symbol = ? AND currency = ? AND status = 'completed'
		 ORDER BY created_at DESC LIMIT 1`,
		symbol, currency,
	).Scan(&id, &model, &status, &macroRaw, &industryRaw, &companyRaw, &internationalRaw,
		&synthesisRaw, &errorMessage, &createdAt, &completedAtRaw)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query symbol analysis: %w", err)
	}

	return buildSymbolAnalysisResult(id, symbol, currency, model, status,
		macroRaw, industryRaw, companyRaw, internationalRaw,
		synthesisRaw, errorMessage, createdAt, completedAtRaw)
}

// GetSymbolAnalysisHistory returns recent completed analyses for a symbol.
func (c *Core) GetSymbolAnalysisHistory(symbol, currency string, limit int) ([]SymbolAnalysisResult, error) {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	currency = strings.TrimSpace(strings.ToUpper(currency))
	if limit <= 0 {
		limit = 10
	}

	rows, err := c.db.Query(
		`SELECT id, model, status, macro_analysis, industry_analysis, company_analysis, international_analysis,
		        synthesis, error_message, created_at, completed_at
		 FROM symbol_analyses
		 WHERE symbol = ? AND currency = ? AND status = 'completed'
		 ORDER BY created_at DESC LIMIT ?`,
		symbol, currency, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query symbol analysis history: %w", err)
	}
	defer rows.Close()

	var results []SymbolAnalysisResult
	for rows.Next() {
		var (
			id               int64
			model, status    string
			macroRaw         sql.NullString
			industryRaw      sql.NullString
			companyRaw       sql.NullString
			internationalRaw sql.NullString
			synthesisRaw     sql.NullString
			errorMessage     sql.NullString
			createdAt        string
			completedAtRaw   sql.NullString
		)
		if err := rows.Scan(&id, &model, &status, &macroRaw, &industryRaw, &companyRaw, &internationalRaw,
			&synthesisRaw, &errorMessage, &createdAt, &completedAtRaw); err != nil {
			return nil, fmt.Errorf("scan symbol analysis row: %w", err)
		}
		result, err := buildSymbolAnalysisResult(id, symbol, currency, model, status,
			macroRaw, industryRaw, companyRaw, internationalRaw,
			synthesisRaw, errorMessage, createdAt, completedAtRaw)
		if err != nil {
			continue
		}
		results = append(results, *result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []SymbolAnalysisResult{}
	}
	return results, nil
}

func buildSymbolAnalysisResult(
	id int64, symbol, currency, model, status string,
	macroRaw, industryRaw, companyRaw, internationalRaw, synthesisRaw, errorMessage sql.NullString,
	createdAt string, completedAtRaw sql.NullString,
) (*SymbolAnalysisResult, error) {
	dimensions := make(map[string]*SymbolDimensionResult)
	dimensionRaws := []struct {
		FallbackKey string
		Raw         sql.NullString
	}{
		{FallbackKey: "macro", Raw: macroRaw},
		{FallbackKey: "industry", Raw: industryRaw},
		{FallbackKey: "company", Raw: companyRaw},
		{FallbackKey: "international", Raw: internationalRaw},
	}
	for _, item := range dimensionRaws {
		if !item.Raw.Valid || strings.TrimSpace(item.Raw.String) == "" {
			continue
		}

		parsed, err := parseSymbolDimensionResult(item.Raw.String)
		if err != nil {
			continue
		}

		dimensionKey := strings.ToLower(strings.TrimSpace(parsed.Dimension))
		if dimensionKey == "" {
			dimensionKey = item.FallbackKey
		}
		normalizeDimensionResult(parsed, dimensionKey)
		dimensions[parsed.Dimension] = parsed
	}

	var synthesis *SymbolSynthesisResult
	if synthesisRaw.Valid && synthesisRaw.String != "" {
		parsed, err := parseSynthesisResult(synthesisRaw.String)
		if err == nil {
			synthesis = parsed
		}
	}
	if synthesis != nil {
		normalizeSynthesisResult(synthesis, nil, orderedDimensionIDs(dimensions))
	}

	completedAt := ""
	if completedAtRaw.Valid {
		completedAt = completedAtRaw.String
	}
	errMsg := ""
	if errorMessage.Valid {
		errMsg = errorMessage.String
	}

	return &SymbolAnalysisResult{
		ID:           id,
		Symbol:       symbol,
		Currency:     currency,
		Model:        model,
		Status:       status,
		Dimensions:   dimensions,
		Synthesis:    synthesis,
		ErrorMessage: errMsg,
		CreatedAt:    createdAt,
		CompletedAt:  completedAt,
	}, nil
}
