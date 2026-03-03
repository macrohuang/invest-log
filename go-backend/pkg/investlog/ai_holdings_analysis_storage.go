package investlog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// saveHoldingsAnalysis persists a completed holdings analysis to the database.
func (c *Core) saveHoldingsAnalysis(result *HoldingsAnalysisResult) (int64, error) {
	findingsJSON, err := json.Marshal(result.KeyFindings)
	if err != nil {
		return 0, fmt.Errorf("marshal key_findings: %w", err)
	}
	recsJSON, err := json.Marshal(result.Recommendations)
	if err != nil {
		return 0, fmt.Errorf("marshal recommendations: %w", err)
	}
	var refsJSON []byte
	if len(result.SymbolRefs) > 0 {
		refsJSON, err = json.Marshal(result.SymbolRefs)
		if err != nil {
			return 0, fmt.Errorf("marshal symbol_refs: %w", err)
		}
	}

	res, err := c.db.Exec(
		`INSERT INTO holdings_analyses
			(currency, model, analysis_type, risk_level, overall_summary, key_findings, recommendations, disclaimer, symbol_refs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.Currency,
		result.Model,
		result.AnalysisType,
		result.RiskLevel,
		result.OverallSummary,
		string(findingsJSON),
		string(recsJSON),
		result.Disclaimer,
		nullableString(string(refsJSON)),
	)
	if err != nil {
		return 0, fmt.Errorf("insert holdings_analysis: %w", err)
	}
	return res.LastInsertId()
}

func nullableString(s string) any {
	if s == "" || s == "null" {
		return nil
	}
	return s
}

// GetHoldingsAnalysis returns the latest saved analysis for the given currency.
func (c *Core) GetHoldingsAnalysis(currency string) (*HoldingsAnalysisResult, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	results, err := c.GetHoldingsAnalysisHistory(currency, 1)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

// GetHoldingsAnalysisHistory returns up to limit recent analyses for the given currency.
func (c *Core) GetHoldingsAnalysisHistory(currency string, limit int) ([]HoldingsAnalysisResult, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if limit <= 0 {
		limit = 10
	}

	var (
		query string
		args  []any
	)
	if currency != "" {
		query = `SELECT id, currency, model, analysis_type, risk_level, overall_summary, key_findings, recommendations, disclaimer, symbol_refs, created_at
		          FROM holdings_analyses WHERE currency = ? ORDER BY created_at DESC LIMIT ?`
		args = []any{currency, limit}
	} else {
		query = `SELECT id, currency, model, analysis_type, risk_level, overall_summary, key_findings, recommendations, disclaimer, symbol_refs, created_at
		          FROM holdings_analyses ORDER BY created_at DESC LIMIT ?`
		args = []any{limit}
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query holdings_analyses: %w", err)
	}
	defer rows.Close()

	var results []HoldingsAnalysisResult
	for rows.Next() {
		var (
			id                        int64
			curr, model, analysisType string
			riskLevel, overallSummary sql.NullString
			keyFindingsRaw, recsRaw   sql.NullString
			disclaimer, symbolRefsRaw sql.NullString
			createdAt                 string
		)
		if err := rows.Scan(&id, &curr, &model, &analysisType, &riskLevel, &overallSummary,
			&keyFindingsRaw, &recsRaw, &disclaimer, &symbolRefsRaw, &createdAt); err != nil {
			return nil, fmt.Errorf("scan holdings_analysis row: %w", err)
		}

		result := HoldingsAnalysisResult{
			ID:             id,
			GeneratedAt:    createdAt,
			Model:          model,
			Currency:       curr,
			AnalysisType:   analysisType,
			RiskLevel:      riskLevel.String,
			OverallSummary: overallSummary.String,
			Disclaimer:     disclaimer.String,
		}

		if keyFindingsRaw.Valid && keyFindingsRaw.String != "" {
			var findings []string
			if err := json.Unmarshal([]byte(keyFindingsRaw.String), &findings); err == nil {
				result.KeyFindings = findings
			}
		}
		if result.KeyFindings == nil {
			result.KeyFindings = []string{}
		}

		if recsRaw.Valid && recsRaw.String != "" {
			var recs []HoldingsAnalysisRecommendation
			if err := json.Unmarshal([]byte(recsRaw.String), &recs); err == nil {
				result.Recommendations = recs
			}
		}
		if result.Recommendations == nil {
			result.Recommendations = []HoldingsAnalysisRecommendation{}
		}

		if symbolRefsRaw.Valid && symbolRefsRaw.String != "" {
			var refs []HoldingsSymbolRef
			if err := json.Unmarshal([]byte(symbolRefsRaw.String), &refs); err == nil {
				result.SymbolRefs = refs
			}
		}

		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []HoldingsAnalysisResult{}
	}
	return results, nil
}
