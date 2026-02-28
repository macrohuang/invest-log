package investlog

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeAllocationAdviceRequest(t *testing.T) {
	t.Parallel()

	t.Run("missing api key", func(t *testing.T) {
		req := AllocationAdviceRequest{Model: "gpt-4o"}
		err := normalizeAllocationAdviceRequest(&req)
		if err == nil || !strings.Contains(err.Error(), "API key is required") {
			t.Fatalf("expected api key validation error, got %v", err)
		}
	})

	t.Run("missing model", func(t *testing.T) {
		req := AllocationAdviceRequest{APIKey: "k"}
		err := normalizeAllocationAdviceRequest(&req)
		if err == nil || !strings.Contains(err.Error(), "model is required") {
			t.Fatalf("expected model validation error, got %v", err)
		}
	})

	t.Run("normalizes fields and currencies", func(t *testing.T) {
		req := AllocationAdviceRequest{
			BaseURL:    "  ",
			APIKey:     " key ",
			Model:      " model ",
			Currencies: []string{" usd ", "XXX", "hkd"},
		}
		if err := normalizeAllocationAdviceRequest(&req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.BaseURL != defaultAIBaseURL {
			t.Fatalf("expected default base url, got %q", req.BaseURL)
		}
		if req.APIKey != "key" || req.Model != "model" {
			t.Fatalf("expected trimmed api key/model, got api_key=%q model=%q", req.APIKey, req.Model)
		}
		if len(req.Currencies) != 2 || req.Currencies[0] != "USD" || req.Currencies[1] != "HKD" {
			t.Fatalf("unexpected normalized currencies: %+v", req.Currencies)
		}
	})

	t.Run("defaults currencies when empty", func(t *testing.T) {
		req := AllocationAdviceRequest{APIKey: "k", Model: "m"}
		if err := normalizeAllocationAdviceRequest(&req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Currencies) != 3 {
			t.Fatalf("expected default 3 currencies, got %+v", req.Currencies)
		}
	})

	t.Run("rejects when all currencies invalid", func(t *testing.T) {
		req := AllocationAdviceRequest{
			APIKey:     "k",
			Model:      "m",
			Currencies: []string{"BTC", "EUR"},
		}
		err := normalizeAllocationAdviceRequest(&req)
		if err == nil || !strings.Contains(err.Error(), "at least one valid currency") {
			t.Fatalf("expected invalid currency error, got %v", err)
		}
	})
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
		CustomPrompt:    "偏好低回撤",
	}
	assetTypes := []AssetType{
		{Code: "stock", Label: "股票"},
		{Code: "bond", Label: "债券"},
	}

	prompt, err := buildAllocationAdviceUserPrompt(req, assetTypes)
	if err != nil {
		t.Fatalf("buildAllocationAdviceUserPrompt failed: %v", err)
	}

	if !strings.Contains(prompt, `"currencies":["USD","CNY"]`) {
		t.Fatalf("expected currencies in prompt payload, got: %s", prompt)
	}
	if !strings.Contains(prompt, `"code":"stock"`) || !strings.Contains(prompt, `"code":"bond"`) {
		t.Fatalf("expected asset types in prompt payload, got: %s", prompt)
	}
	if !strings.Contains(prompt, `共 4 个组合`) {
		t.Fatalf("expected combinations count in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "偏好低回撤") {
		t.Fatalf("expected custom prompt in payload, got: %s", prompt)
	}
}

func TestParseAllocationAdviceResponse(t *testing.T) {
	t.Parallel()

	content := "```json\n{\"summary\":\"s\",\"rationale\":\"r\",\"allocations\":[{\"currency\":\"USD\",\"asset_type\":\"stock\",\"label\":\"股票\",\"min_percent\":10,\"max_percent\":30,\"rationale\":\"x\"}],\"disclaimer\":\"d\"}\n```"
	parsed, err := parseAllocationAdviceResponse(content)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.Summary != "s" {
		t.Fatalf("unexpected summary: %s", parsed.Summary)
	}
	if len(parsed.Allocations) != 1 {
		t.Fatalf("expected one allocation, got %d", len(parsed.Allocations))
	}

	_, err = parseAllocationAdviceResponse("{")
	if err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
}

func TestClampPercent(t *testing.T) {
	t.Parallel()

	if got := clampPercent(-1); got != 0 {
		t.Fatalf("expected 0 for negative value, got %v", got)
	}
	if got := clampPercent(130); got != 100 {
		t.Fatalf("expected 100 for value over 100, got %v", got)
	}
	if got := clampPercent(33.3); got != 33.3 {
		t.Fatalf("expected unchanged value, got %v", got)
	}
}

func TestGetAllocationAdvice_EndToEndWithStub(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		if !strings.Contains(req.EndpointURL, "/chat/completions") {
			t.Fatalf("unexpected endpoint: %s", req.EndpointURL)
		}
		if !strings.Contains(req.UserPrompt, `"currencies":["USD"]`) {
			t.Fatalf("unexpected user prompt: %s", req.UserPrompt)
		}
		return aiChatCompletionResult{
			Model: "mock-allocation-model",
			Content: `{
				"summary":"整体均衡配置",
				"rationale":"根据风险偏好平衡权益与债券",
				"allocations":[
					{"currency":"usd","asset_type":"stock","label":"股票","min_percent":70,"max_percent":40,"rationale":"成长驱动"},
					{"currency":"USD","asset_type":"bond","label":"债券","min_percent":-10,"max_percent":120,"rationale":"稳定器"}
				],
				"disclaimer":"仅供参考"
			}`,
		}, nil
	}

	result, err := core.GetAllocationAdvice(AllocationAdviceRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "mock-allocation-model",
		Currencies:      []string{"USD"},
		AgeRange:        "30s",
		InvestGoal:      "balanced",
		RiskTolerance:   "balanced",
		Horizon:         "long",
		ExperienceLevel: "intermediate",
	})
	if err != nil {
		t.Fatalf("GetAllocationAdvice failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Model != "mock-allocation-model" {
		t.Fatalf("unexpected model: %s", result.Model)
	}
	if result.GeneratedAt == "" {
		t.Fatal("expected generated_at to be set")
	}
	if len(result.Allocations) != 2 {
		t.Fatalf("expected 2 allocations, got %d", len(result.Allocations))
	}

	first := result.Allocations[0]
	if first.Currency != "USD" {
		t.Fatalf("expected normalized currency USD, got %s", first.Currency)
	}
	if first.MinPercent != 40 || first.MaxPercent != 70 {
		t.Fatalf("expected swapped range [40,70], got [%v,%v]", first.MinPercent, first.MaxPercent)
	}

	second := result.Allocations[1]
	if second.MinPercent != 0 || second.MaxPercent != 100 {
		t.Fatalf("expected clamped range [0,100], got [%v,%v]", second.MinPercent, second.MaxPercent)
	}
}

func TestGetAllocationAdviceWithStream_EmitsDelta(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		if req.OnDelta == nil {
			t.Fatal("expected onDelta callback")
		}
		req.OnDelta("第一段")
		req.OnDelta("第二段")
		return aiChatCompletionResult{
			Model: "mock-allocation-model",
			Content: `{
				"summary":"s",
				"rationale":"r",
				"allocations":[{"currency":"USD","asset_type":"stock","label":"股票","min_percent":10,"max_percent":30,"rationale":"x"}],
				"disclaimer":"仅供参考"
			}`,
		}, nil
	}

	var streamed strings.Builder
	result, err := core.GetAllocationAdviceWithStream(AllocationAdviceRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "mock-allocation-model",
		Currencies:      []string{"USD"},
		AgeRange:        "30s",
		InvestGoal:      "balanced",
		RiskTolerance:   "balanced",
		Horizon:         "long",
		ExperienceLevel: "intermediate",
	}, func(delta string) {
		streamed.WriteString(delta)
	})
	if err != nil {
		t.Fatalf("GetAllocationAdviceWithStream failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if streamed.String() != "第一段第二段" {
		t.Fatalf("unexpected streamed deltas: %q", streamed.String())
	}
}
