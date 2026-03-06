package investlog

import (
	"context"
	"fmt"
	"strings"
)

// RunAIAnalysis executes one generic AI analysis method without streaming.
func (c *Core) RunAIAnalysis(req RunAIAnalysisRequest) (*AIAnalysisRun, error) {
	return c.runAIAnalysis(req, nil)
}

// RunAIAnalysisStream executes one generic AI analysis method and emits delta chunks.
func (c *Core) RunAIAnalysisStream(req RunAIAnalysisRequest, onDelta func(string) error) (*AIAnalysisRun, error) {
	return c.runAIAnalysis(req, onDelta)
}

func (c *Core) runAIAnalysis(req RunAIAnalysisRequest, onDelta func(string) error) (*AIAnalysisRun, error) {
	method, settings, renderedSystemPrompt, renderedUserPrompt, normalizedVars, err := c.prepareAIAnalysis(req)
	if err != nil {
		return nil, err
	}

	run := AIAnalysisRun{
		MethodID:             int64Ptr(method.ID),
		MethodName:           method.Name,
		SystemPromptTemplate: method.SystemPrompt,
		UserPromptTemplate:   method.UserPrompt,
		Variables:            normalizedVars,
		RenderedSystemPrompt: renderedSystemPrompt,
		RenderedUserPrompt:   renderedUserPrompt,
		Model:                settings.Model,
		Status:               "running",
	}

	runID, err := c.insertAIAnalysisRun(run)
	if err != nil {
		return nil, err
	}

	endpointURL, err := buildAICompletionsEndpoint(settings.BaseURL)
	if err != nil {
		_ = c.completeAIAnalysisRun(runID, "failed", "", err.Error())
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), aiTotalRequestTimeout)
	defer cancel()

	chatReq := aiChatCompletionRequest{
		EndpointURL:         endpointURL,
		APIKey:              settings.APIKey,
		Model:               settings.Model,
		SystemPrompt:        renderedSystemPrompt,
		UserPrompt:          renderedUserPrompt,
		Logger:              c.Logger(),
		UseGoogleSearchTool: true,
	}

	var result aiChatCompletionResult
	if onDelta != nil {
		result, err = aiChatCompletionStream(ctx, chatReq, onDelta)
	} else {
		result, err = aiChatCompletion(ctx, chatReq)
	}
	if err != nil {
		_ = c.completeAIAnalysisRun(runID, "failed", "", err.Error())
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	if err := c.completeAIAnalysisRun(runID, "completed", result.Content, ""); err != nil {
		return nil, err
	}

	saved, err := c.GetAIAnalysisRun(runID)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return nil, fmt.Errorf("ai analysis run not found after completion")
	}
	saved.Model = result.Model
	if result.Model != "" && result.Model != run.Model {
		_, err = c.db.Exec(`UPDATE ai_analysis_runs SET model = ? WHERE id = ?`, result.Model, runID)
		if err != nil {
			return nil, fmt.Errorf("update ai analysis run model: %w", err)
		}
		saved.Model = result.Model
	}
	return saved, nil
}

func (c *Core) prepareAIAnalysis(req RunAIAnalysisRequest) (*AIAnalysisMethod, AISettings, string, string, map[string]string, error) {
	if req.MethodID <= 0 {
		return nil, AISettings{}, "", "", nil, fmt.Errorf("method_id is required")
	}

	method, err := c.GetAIAnalysisMethod(req.MethodID)
	if err != nil {
		return nil, AISettings{}, "", "", nil, err
	}
	if method == nil {
		return nil, AISettings{}, "", "", nil, fmt.Errorf("ai analysis method not found")
	}

	settings, err := c.GetAISettings()
	if err != nil {
		return nil, AISettings{}, "", "", nil, err
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return nil, AISettings{}, "", "", nil, fmt.Errorf("AI API key is required")
	}
	if strings.TrimSpace(settings.Model) == "" {
		return nil, AISettings{}, "", "", nil, fmt.Errorf("AI model is required")
	}

	renderedSystemPrompt, normalizedVars, err := renderAIAnalysisPrompt(method.SystemPrompt, method.Variables, req.Variables)
	if err != nil {
		return nil, AISettings{}, "", "", nil, err
	}
	renderedUserPrompt, _, err := renderAIAnalysisPrompt(method.UserPrompt, method.Variables, req.Variables)
	if err != nil {
		return nil, AISettings{}, "", "", nil, err
	}

	return method, settings, renderedSystemPrompt, renderedUserPrompt, normalizedVars, nil
}

func renderAIAnalysisPrompt(template string, expectedVars []string, providedVars map[string]string) (string, map[string]string, error) {
	if providedVars == nil {
		providedVars = map[string]string{}
	}

	normalized := make(map[string]string, len(providedVars))
	for key, value := range providedVars {
		normalized[strings.TrimSpace(key)] = value
	}

	rendered := template
	for _, name := range expectedVars {
		value, ok := normalized[name]
		if !ok {
			return "", nil, fmt.Errorf("missing variable: %s", name)
		}
		rendered = strings.ReplaceAll(rendered, "${"+name+"}", value)
	}
	return rendered, normalized, nil
}

func int64Ptr(value int64) *int64 {
	return &value
}
