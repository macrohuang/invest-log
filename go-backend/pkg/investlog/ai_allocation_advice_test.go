package investlog

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeAllocationAdviceRequest(t *testing.T) {
	t.Parallel()

	req := AllocationAdviceRequest{APIKey: " key ", Model: " model ", Currencies: []string{"usd", "EUR", "hkd"}}
	if err := normalizeAllocationAdviceRequest(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.APIKey != "key" || req.Model != "model" {
		t.Fatalf("expected trimmed fields, got %+v", req)
	}
	if len(req.Currencies) != 2 || req.Currencies[0] != "USD" || req.Currencies[1] != "HKD" {
		t.Fatalf("unexpected currencies: %+v", req.Currencies)
	}

	req = AllocationAdviceRequest{Model: "m"}
	if err := normalizeAllocationAdviceRequest(&req); err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected api key error, got %v", err)
	}
}

func TestBuildAndParseAllocationAdvicePrompt(t *testing.T) {
	t.Parallel()

	prompt, err := buildAllocationAdviceUserPrompt(AllocationAdviceRequest{
		AgeRange:        "30s",
		InvestGoal:      "growth",
		RiskTolerance:   "balanced",
		Horizon:         "long",
		ExperienceLevel: "intermediate",
		Currencies:      []string{"USD", "CNY"},
		CustomPrompt:    "更关注回撤",
	}, []AssetType{{Code: "stock", Label: "Stock"}, {Code: "bond", Label: "Bond"}})
	if err != nil {
		t.Fatalf("buildAllocationAdviceUserPrompt failed: %v", err)
	}
	if !strings.Contains(prompt, "共 4 个组合") {
		t.Fatalf("expected combinations hint in prompt, got: %s", prompt)
	}

	parsed, err := parseAllocationAdviceResponse(`{"summary":"s","rationale":"r","allocations":[{"currency":"USD","asset_type":"stock","label":"Stock","min_percent":10,"max_percent":60,"rationale":"x"}],"disclaimer":"d"}`)
	if err != nil {
		t.Fatalf("parseAllocationAdviceResponse failed: %v", err)
	}
	if parsed.Summary != "s" || len(parsed.Allocations) != 1 {
		t.Fatalf("unexpected parsed result: %+v", parsed)
	}
}

func TestClampPercent(t *testing.T) {
	t.Parallel()

	if got := clampPercent(-10); got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
	if got := clampPercent(120); got != 100 {
		t.Fatalf("expected 100, got %v", got)
	}
	if got := clampPercent(35.5); got != 35.5 {
		t.Fatalf("expected unchanged value, got %v", got)
	}
}

func TestGetAllocationAdviceWithStub(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		return aiChatCompletionResult{
			Model: "gemini-2.5-flash",
			Content: `{
				"summary":"组合以均衡为主",
				"rationale":"根据风险偏好给出区间",
				"allocations":[
					{"currency":"USD","asset_type":"stock","label":"Stock","min_percent":20,"max_percent":70,"rationale":"长期增长"}
				],
				"disclaimer":"仅供参考"
			}`,
		}, nil
	}

	result, err := core.GetAllocationAdvice(AllocationAdviceRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "gemini-2.5-flash",
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
	if result == nil || len(result.Allocations) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Allocations[0].Currency != "USD" {
		t.Fatalf("unexpected allocation currency: %+v", result.Allocations[0])
	}
}
