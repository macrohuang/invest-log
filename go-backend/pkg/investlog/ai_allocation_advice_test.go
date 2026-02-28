package investlog

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeAllocationAdviceRequest(t *testing.T) {
	t.Parallel()

	req := AllocationAdviceRequest{
		Model: "m",
	}
	if err := normalizeAllocationAdviceRequest(&req); err == nil {
		t.Fatal("expected missing api key error")
	}

	req = AllocationAdviceRequest{
		APIKey:     " key ",
		Model:      " model ",
		Currencies: []string{"usd", "eur", "HKD"},
	}
	if err := normalizeAllocationAdviceRequest(&req); err != nil {
		t.Fatalf("unexpected normalize error: %v", err)
	}
	if req.APIKey != "key" {
		t.Fatalf("expected trimmed api key, got %q", req.APIKey)
	}
	if req.Model != "model" {
		t.Fatalf("expected trimmed model, got %q", req.Model)
	}
	if len(req.Currencies) != 2 || req.Currencies[0] != "USD" || req.Currencies[1] != "HKD" {
		t.Fatalf("unexpected currencies: %+v", req.Currencies)
	}
}

func TestBuildAllocationAdviceUserPrompt(t *testing.T) {
	t.Parallel()

	req := AllocationAdviceRequest{
		AgeRange:        "30s",
		InvestGoal:      "growth",
		RiskTolerance:   "balanced",
		Horizon:         "long",
		ExperienceLevel: "intermediate",
		Currencies:      []string{"USD", "CNY"},
		CustomPrompt:    "偏向科技",
	}
	assetTypes := []AssetType{
		{Code: "stock", Label: "股票"},
		{Code: "bond", Label: "债券"},
	}

	prompt, err := buildAllocationAdviceUserPrompt(req, assetTypes)
	if err != nil {
		t.Fatalf("unexpected build prompt error: %v", err)
	}
	if !strings.Contains(prompt, "偏向科技") {
		t.Fatalf("expected custom prompt in output: %s", prompt)
	}
	if !strings.Contains(prompt, "4 个组合") {
		t.Fatalf("expected currency*asset count in prompt: %s", prompt)
	}
}

func TestParseAllocationAdviceResponseAndClampPercent(t *testing.T) {
	t.Parallel()

	content := "```json\n{\"summary\":\"s\",\"rationale\":\"r\",\"allocations\":[{\"currency\":\"USD\",\"asset_type\":\"stock\",\"label\":\"股票\",\"min_percent\":-5,\"max_percent\":120,\"rationale\":\"x\"}],\"disclaimer\":\"d\"}\n```"
	parsed, err := parseAllocationAdviceResponse(content)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.Summary != "s" || parsed.Rationale != "r" {
		t.Fatalf("unexpected parsed response: %+v", parsed)
	}

	if got := clampPercent(-1); got != 0 {
		t.Fatalf("expected clamp 0, got %v", got)
	}
	if got := clampPercent(120); got != 100 {
		t.Fatalf("expected clamp 100, got %v", got)
	}
	if got := clampPercent(35.5); got != 35.5 {
		t.Fatalf("expected passthrough value, got %v", got)
	}
}

func TestGetAllocationAdvice_WithStub(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()
	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		return aiChatCompletionResult{
			Model: "mock-allocation-model",
			Content: `{
				"summary":"配置建议摘要",
				"rationale":"风险收益平衡",
				"allocations":[
					{"currency":"USD","asset_type":"stock","label":"股票","min_percent":10,"max_percent":70,"rationale":"长期增长"},
					{"currency":"USD","asset_type":"bond","label":"债券","min_percent":20,"max_percent":80,"rationale":"降低波动"}
				],
				"disclaimer":"仅供参考"
			}`,
		}, nil
	}

	result, err := core.GetAllocationAdvice(AllocationAdviceRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "mock-model",
		AgeRange:        "30s",
		InvestGoal:      "growth",
		RiskTolerance:   "balanced",
		Horizon:         "long",
		ExperienceLevel: "intermediate",
		Currencies:      []string{"USD"},
	})
	if err != nil {
		t.Fatalf("GetAllocationAdvice failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Model != "mock-allocation-model" {
		t.Fatalf("unexpected model: %s", result.Model)
	}
	if len(result.Allocations) == 0 {
		t.Fatal("expected allocations")
	}
}
