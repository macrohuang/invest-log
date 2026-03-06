package investlog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// AIAnalysisRun represents one persisted execution of a configured method.
type AIAnalysisRun struct {
	ID                   int64             `json:"id"`
	MethodID             *int64            `json:"method_id,omitempty"`
	MethodName           string            `json:"method_name"`
	SystemPromptTemplate string            `json:"system_prompt_template"`
	UserPromptTemplate   string            `json:"user_prompt_template"`
	Variables            map[string]string `json:"variables"`
	RenderedSystemPrompt string            `json:"rendered_system_prompt"`
	RenderedUserPrompt   string            `json:"rendered_user_prompt"`
	Model                string            `json:"model"`
	Status               string            `json:"status"`
	ResultText           string            `json:"result_text,omitempty"`
	ErrorMessage         string            `json:"error_message,omitempty"`
	CreatedAt            string            `json:"created_at"`
	CompletedAt          string            `json:"completed_at,omitempty"`
}

func (c *Core) insertAIAnalysisRun(run AIAnalysisRun) (int64, error) {
	variablesJSON, err := json.Marshal(run.Variables)
	if err != nil {
		return 0, fmt.Errorf("marshal ai analysis run variables: %w", err)
	}

	var methodID any
	if run.MethodID != nil {
		methodID = *run.MethodID
	}

	res, err := c.db.Exec(`
		INSERT INTO ai_analysis_runs (
			method_id, method_name, system_prompt_template, user_prompt_template,
			variables_json, rendered_system_prompt, rendered_user_prompt, model, status
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, methodID, run.MethodName, run.SystemPromptTemplate, run.UserPromptTemplate,
		string(variablesJSON), run.RenderedSystemPrompt, run.RenderedUserPrompt, run.Model, run.Status)
	if err != nil {
		return 0, fmt.Errorf("insert ai analysis run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get ai analysis run id: %w", err)
	}
	return id, nil
}

func (c *Core) completeAIAnalysisRun(id int64, status, resultText, errorMessage string) error {
	_, err := c.db.Exec(`
		UPDATE ai_analysis_runs
		SET status = ?, result_text = ?, error_message = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, nullableString(resultText), nullableString(errorMessage), id)
	if err != nil {
		return fmt.Errorf("update ai analysis run: %w", err)
	}
	return nil
}

// ListAIAnalysisRuns returns recent generic AI analysis runs.
func (c *Core) ListAIAnalysisRuns(methodID int64, limit int) ([]AIAnalysisRun, error) {
	if limit <= 0 {
		limit = 10
	}

	var (
		rows *sql.Rows
		err  error
	)
	if methodID > 0 {
		rows, err = c.db.Query(`
			SELECT id, method_id, method_name, system_prompt_template, user_prompt_template,
			       variables_json, rendered_system_prompt, rendered_user_prompt, model,
			       status, result_text, error_message, created_at, completed_at
			FROM ai_analysis_runs
			WHERE method_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		`, methodID, limit)
	} else {
		rows, err = c.db.Query(`
			SELECT id, method_id, method_name, system_prompt_template, user_prompt_template,
			       variables_json, rendered_system_prompt, rendered_user_prompt, model,
			       status, result_text, error_message, created_at, completed_at
			FROM ai_analysis_runs
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("query ai analysis runs: %w", err)
	}
	defer rows.Close()

	var runs []AIAnalysisRun
	for rows.Next() {
		run, err := scanAIAnalysisRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai analysis runs: %w", err)
	}
	if runs == nil {
		runs = []AIAnalysisRun{}
	}
	return runs, nil
}

// GetAIAnalysisRun returns one run detail.
func (c *Core) GetAIAnalysisRun(id int64) (*AIAnalysisRun, error) {
	row := c.db.QueryRow(`
		SELECT id, method_id, method_name, system_prompt_template, user_prompt_template,
		       variables_json, rendered_system_prompt, rendered_user_prompt, model,
		       status, result_text, error_message, created_at, completed_at
		FROM ai_analysis_runs
		WHERE id = ?
	`, id)

	run, err := scanAIAnalysisRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

type aiAnalysisRunScanner interface {
	Scan(dest ...any) error
}

func scanAIAnalysisRun(scanner aiAnalysisRunScanner) (AIAnalysisRun, error) {
	var (
		run             AIAnalysisRun
		methodIDRaw     sql.NullInt64
		variablesJSON   string
		resultTextRaw   sql.NullString
		errorMessageRaw sql.NullString
		completedAtRaw  sql.NullString
	)
	err := scanner.Scan(
		&run.ID,
		&methodIDRaw,
		&run.MethodName,
		&run.SystemPromptTemplate,
		&run.UserPromptTemplate,
		&variablesJSON,
		&run.RenderedSystemPrompt,
		&run.RenderedUserPrompt,
		&run.Model,
		&run.Status,
		&resultTextRaw,
		&errorMessageRaw,
		&run.CreatedAt,
		&completedAtRaw,
	)
	if err != nil {
		return AIAnalysisRun{}, err
	}

	if methodIDRaw.Valid {
		run.MethodID = &methodIDRaw.Int64
	}
	run.Variables = map[string]string{}
	if strings.TrimSpace(variablesJSON) != "" {
		if err := json.Unmarshal([]byte(variablesJSON), &run.Variables); err != nil {
			return AIAnalysisRun{}, fmt.Errorf("unmarshal ai analysis run variables: %w", err)
		}
	}
	if resultTextRaw.Valid {
		run.ResultText = resultTextRaw.String
	}
	if errorMessageRaw.Valid {
		run.ErrorMessage = errorMessageRaw.String
	}
	if completedAtRaw.Valid {
		run.CompletedAt = completedAtRaw.String
	}
	return run, nil
}
