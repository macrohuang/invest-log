package investlog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractAIAnalysisVariables(t *testing.T) {
	vars := extractAIAnalysisVariables(
		"System prompt for ${SYMBOL} vs ${BENCHMARK}",
		"Explain ${SYMBOL} in ${LANGUAGE} and compare with ${BENCHMARK}. Ignore ${symbol}.",
	)

	want := []string{"SYMBOL", "BENCHMARK", "LANGUAGE"}
	if len(vars) != len(want) {
		t.Fatalf("expected %d vars, got %d: %#v", len(want), len(vars), vars)
	}
	for i, item := range want {
		if vars[i] != item {
			t.Fatalf("unexpected var at %d: got %q want %q", i, vars[i], item)
		}
	}
}

func TestAIAnalysisMethodCRUD(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := core.CreateAIAnalysisMethod(AIAnalysisMethod{
		Name:         "估值复盘",
		SystemPrompt: "你是分析师，关注 ${SYMBOL}",
		UserPrompt:   "请用 ${LANGUAGE} 输出 ${SYMBOL} 的估值判断",
	})
	assertNoError(t, err, "create ai analysis method")

	if created.ID <= 0 {
		t.Fatalf("expected created ID > 0, got %d", created.ID)
	}
	if len(created.Variables) != 2 {
		t.Fatalf("expected 2 variables, got %#v", created.Variables)
	}

	list, err := core.ListAIAnalysisMethods()
	assertNoError(t, err, "list ai analysis methods")
	if len(list) != 1 {
		t.Fatalf("expected 1 method, got %d", len(list))
	}

	loaded, err := core.GetAIAnalysisMethod(created.ID)
	assertNoError(t, err, "get ai analysis method")
	if loaded == nil {
		t.Fatal("expected method to exist")
	}
	if loaded.Name != created.Name {
		t.Fatalf("unexpected loaded name: %q", loaded.Name)
	}

	updated, err := core.UpdateAIAnalysisMethod(created.ID, AIAnalysisMethod{
		Name:         "财报拆解",
		SystemPrompt: "分析 ${TICKER}",
		UserPrompt:   "回答 ${QUESTION}",
	})
	assertNoError(t, err, "update ai analysis method")
	if updated.Name != "财报拆解" {
		t.Fatalf("unexpected updated name: %q", updated.Name)
	}
	if len(updated.Variables) != 2 || updated.Variables[0] != "TICKER" || updated.Variables[1] != "QUESTION" {
		t.Fatalf("unexpected updated variables: %#v", updated.Variables)
	}

	deleted, err := core.DeleteAIAnalysisMethod(created.ID)
	assertNoError(t, err, "delete ai analysis method")
	if !deleted {
		t.Fatal("expected delete to return true")
	}

	afterDelete, err := core.ListAIAnalysisMethods()
	assertNoError(t, err, "list methods after delete")
	if len(afterDelete) != 0 {
		t.Fatalf("expected empty methods after delete, got %d", len(afterDelete))
	}
}

func TestRunAIAnalysisStreamPersistsHistory(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		tools, ok := reqBody["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("expected tools array, got %#v", reqBody["tools"])
		}
		toolEntry, ok := tools[0].(map[string]any)
		if !ok {
			t.Fatalf("expected tool entry object, got %#v", tools[0])
		}
		searchConfig, ok := toolEntry["google_search"].(map[string]any)
		if !ok {
			t.Fatalf("expected google_search tool config, got %#v", toolEntry)
		}
		if len(searchConfig) != 0 {
			t.Fatalf("expected empty google_search config, got %#v", searchConfig)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"mock-model","choices":[{"message":{"content":"Final analysis for AAPL"}}]}`))
	}))
	defer server.Close()

	_, err := core.SetAISettings(AISettings{
		BaseURL: server.URL,
		Model:   "mock-model",
		APIKey:  "test-key",
	})
	assertNoError(t, err, "set ai settings")

	method, err := core.CreateAIAnalysisMethod(AIAnalysisMethod{
		Name:         "股票速览",
		SystemPrompt: "你是研究员，请分析 ${SYMBOL}",
		UserPrompt:   "请回答问题：${QUESTION}",
	})
	assertNoError(t, err, "create method")

	var streamed strings.Builder
	result, err := core.RunAIAnalysisStream(RunAIAnalysisRequest{
		MethodID: method.ID,
		Variables: map[string]string{
			"SYMBOL":   "AAPL",
			"QUESTION": "增长是否可持续",
		},
	}, func(delta string) error {
		streamed.WriteString(delta)
		return nil
	})
	assertNoError(t, err, "run ai analysis stream")

	if result.Status != "completed" {
		t.Fatalf("unexpected result status: %q", result.Status)
	}
	if result.ResultText != "Final analysis for AAPL" {
		t.Fatalf("unexpected result text: %q", result.ResultText)
	}
	if !strings.Contains(result.RenderedSystemPrompt, "AAPL") {
		t.Fatalf("expected rendered system prompt to contain replacement, got %q", result.RenderedSystemPrompt)
	}
	if streamed.String() != "Final analysis for AAPL" {
		t.Fatalf("unexpected streamed content: %q", streamed.String())
	}

	history, err := core.ListAIAnalysisRuns(method.ID, 10)
	assertNoError(t, err, "list ai analysis runs")
	if len(history) != 1 {
		t.Fatalf("expected 1 run, got %d", len(history))
	}
	if history[0].MethodName != "股票速览" {
		t.Fatalf("unexpected method name in history: %q", history[0].MethodName)
	}
	if history[0].Variables["SYMBOL"] != "AAPL" {
		t.Fatalf("unexpected variables in history: %#v", history[0].Variables)
	}
}
