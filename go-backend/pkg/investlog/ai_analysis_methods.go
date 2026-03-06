package investlog

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var aiAnalysisVariablePattern = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

// AIAnalysisMethod represents one configurable prompt template.
type AIAnalysisMethod struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	UserPrompt   string   `json:"user_prompt"`
	Variables    []string `json:"variables"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// RunAIAnalysisRequest defines the inputs for one generic AI analysis run.
type RunAIAnalysisRequest struct {
	MethodID  int64
	Variables map[string]string
}

func extractAIAnalysisVariables(systemPrompt, userPrompt string) []string {
	matches := aiAnalysisVariablePattern.FindAllStringSubmatch(systemPrompt+"\n"+userPrompt, -1)
	if len(matches) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(matches))
	var vars []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		vars = append(vars, name)
	}
	return vars
}

func normalizeAIAnalysisMethod(method AIAnalysisMethod) AIAnalysisMethod {
	method.Name = strings.TrimSpace(method.Name)
	method.SystemPrompt = strings.TrimSpace(method.SystemPrompt)
	method.UserPrompt = strings.TrimSpace(method.UserPrompt)
	method.Variables = extractAIAnalysisVariables(method.SystemPrompt, method.UserPrompt)
	return method
}

func validateAIAnalysisMethod(method AIAnalysisMethod) error {
	if method.Name == "" {
		return fmt.Errorf("name is required")
	}
	if method.SystemPrompt == "" {
		return fmt.Errorf("system_prompt is required")
	}
	if method.UserPrompt == "" {
		return fmt.Errorf("user_prompt is required")
	}
	return nil
}

// ListAIAnalysisMethods returns all configured methods.
func (c *Core) ListAIAnalysisMethods() ([]AIAnalysisMethod, error) {
	rows, err := c.db.Query(`
		SELECT id, name, system_prompt, user_prompt, created_at, updated_at
		FROM ai_analysis_methods
		ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query ai analysis methods: %w", err)
	}
	defer rows.Close()

	var methods []AIAnalysisMethod
	for rows.Next() {
		method, err := scanAIAnalysisMethod(rows)
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai analysis methods: %w", err)
	}
	if methods == nil {
		methods = []AIAnalysisMethod{}
	}
	return methods, nil
}

// GetAIAnalysisMethod returns one configured method.
func (c *Core) GetAIAnalysisMethod(id int64) (*AIAnalysisMethod, error) {
	row := c.db.QueryRow(`
		SELECT id, name, system_prompt, user_prompt, created_at, updated_at
		FROM ai_analysis_methods
		WHERE id = ?
	`, id)

	method, err := scanAIAnalysisMethod(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &method, nil
}

// CreateAIAnalysisMethod persists a new analysis method.
func (c *Core) CreateAIAnalysisMethod(method AIAnalysisMethod) (AIAnalysisMethod, error) {
	normalized := normalizeAIAnalysisMethod(method)
	if err := validateAIAnalysisMethod(normalized); err != nil {
		return AIAnalysisMethod{}, err
	}

	res, err := c.db.Exec(`
		INSERT INTO ai_analysis_methods (name, system_prompt, user_prompt)
		VALUES (?, ?, ?)
	`, normalized.Name, normalized.SystemPrompt, normalized.UserPrompt)
	if err != nil {
		if isUniqueConstraintError(err, "ai_analysis_methods.name") {
			return AIAnalysisMethod{}, fmt.Errorf("ai analysis method name already exists")
		}
		return AIAnalysisMethod{}, fmt.Errorf("insert ai analysis method: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return AIAnalysisMethod{}, fmt.Errorf("get inserted ai analysis method id: %w", err)
	}
	created, err := c.GetAIAnalysisMethod(id)
	if err != nil {
		return AIAnalysisMethod{}, err
	}
	if created == nil {
		return AIAnalysisMethod{}, fmt.Errorf("ai analysis method not found after insert")
	}
	return *created, nil
}

// UpdateAIAnalysisMethod updates an existing method.
func (c *Core) UpdateAIAnalysisMethod(id int64, method AIAnalysisMethod) (AIAnalysisMethod, error) {
	normalized := normalizeAIAnalysisMethod(method)
	if err := validateAIAnalysisMethod(normalized); err != nil {
		return AIAnalysisMethod{}, err
	}

	res, err := c.db.Exec(`
		UPDATE ai_analysis_methods
		SET name = ?, system_prompt = ?, user_prompt = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, normalized.Name, normalized.SystemPrompt, normalized.UserPrompt, id)
	if err != nil {
		if isUniqueConstraintError(err, "ai_analysis_methods.name") {
			return AIAnalysisMethod{}, fmt.Errorf("ai analysis method name already exists")
		}
		return AIAnalysisMethod{}, fmt.Errorf("update ai analysis method: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return AIAnalysisMethod{}, fmt.Errorf("get updated ai analysis method rows: %w", err)
	}
	if affected == 0 {
		return AIAnalysisMethod{}, fmt.Errorf("ai analysis method not found")
	}

	updated, err := c.GetAIAnalysisMethod(id)
	if err != nil {
		return AIAnalysisMethod{}, err
	}
	if updated == nil {
		return AIAnalysisMethod{}, fmt.Errorf("ai analysis method not found after update")
	}
	return *updated, nil
}

// DeleteAIAnalysisMethod deletes a method by id.
func (c *Core) DeleteAIAnalysisMethod(id int64) (bool, error) {
	res, err := c.db.Exec(`DELETE FROM ai_analysis_methods WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete ai analysis method: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("get deleted ai analysis method rows: %w", err)
	}
	return affected > 0, nil
}

type aiAnalysisMethodScanner interface {
	Scan(dest ...any) error
}

func scanAIAnalysisMethod(scanner aiAnalysisMethodScanner) (AIAnalysisMethod, error) {
	var method AIAnalysisMethod
	err := scanner.Scan(
		&method.ID,
		&method.Name,
		&method.SystemPrompt,
		&method.UserPrompt,
		&method.CreatedAt,
		&method.UpdatedAt,
	)
	if err != nil {
		return AIAnalysisMethod{}, err
	}
	method = normalizeAIAnalysisMethod(method)
	return method, nil
}

func isUniqueConstraintError(err error, key string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") && strings.Contains(message, strings.ToLower(key))
}
