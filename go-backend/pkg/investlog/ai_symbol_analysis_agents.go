package investlog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type frameworkAgent struct {
	FrameworkID  string
	SystemPrompt string
}

type agentResult struct {
	FrameworkID string
	Content     string
	Error       error
}

func (c *Core) runDimensionAgents(
	ctx context.Context,
	endpoint, apiKey, model string,
	frameworks []symbolFrameworkSpec,
	userPrompt string,
	onDelta func(string),
) (map[string]string, error) {
	if len(frameworks) < minFrameworkAnalyses {
		return nil, fmt.Errorf("selected frameworks less than %d", minFrameworkAnalyses)
	}

	agents := make([]frameworkAgent, 0, len(frameworks))
	for _, framework := range frameworks {
		agents = append(agents, frameworkAgent{
			FrameworkID:  framework.ID,
			SystemPrompt: buildFrameworkSystemPrompt(framework),
		})
	}

	ch := make(chan agentResult, len(agents))
	var wg sync.WaitGroup

	for _, a := range agents {
		wg.Add(1)
		go func(frameworkID, sysPrompt string) {
			defer wg.Done()
			res, err := aiChatCompletion(ctx, aiChatCompletionRequest{
				EndpointURL:  endpoint,
				APIKey:       apiKey,
				Model:        model,
				SystemPrompt: sysPrompt,
				UserPrompt:   userPrompt,
				Logger:       c.Logger(),
				OnDelta: func(delta string) {
					delta = strings.TrimSpace(delta)
					if delta == "" || onDelta == nil {
						return
					}
					onDelta("[" + frameworkID + "] " + delta)
				},
			})
			if err != nil {
				ch <- agentResult{FrameworkID: frameworkID, Error: err}
				return
			}
			ch <- agentResult{FrameworkID: frameworkID, Content: res.Content}
		}(a.FrameworkID, a.SystemPrompt)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	outputs := make(map[string]string, len(agents))
	var errs []string
	for r := range ch {
		if r.Error != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.FrameworkID, r.Error))
			continue
		}
		outputs[r.FrameworkID] = r.Content
	}

	if len(outputs) < minFrameworkAnalyses {
		return nil, fmt.Errorf("framework analyses insufficient (%d/%d): %s", len(outputs), len(agents), strings.Join(errs, "; "))
	}
	return outputs, nil
}

func runSynthesisAgent(
	ctx context.Context,
	endpoint, apiKey, model, symbolContext string,
	frameworkOutputs map[string]string,
	frameworkIDs []string,
	weightContext symbolSynthesisWeightContext,
	onDelta func(string),
) (string, error) {
	frameworkJSON, err := json.Marshal(frameworkOutputs)
	if err != nil {
		return "", fmt.Errorf("marshal framework outputs: %w", err)
	}
	frameworkIDsJSON, err := json.Marshal(frameworkIDs)
	if err != nil {
		return "", fmt.Errorf("marshal framework ids: %w", err)
	}
	weightJSON, err := json.Marshal(weightContext)
	if err != nil {
		return "", fmt.Errorf("marshal synthesis weight context: %w", err)
	}

	userPrompt := fmt.Sprintf(`请基于以下信息给出综合建议：

标的信息：
%s

三框架ID（必须逐一引用）：
%s

三框架分析结果：
%s

权重上下文（必须纳入计算）：
%s

硬约束：
1) overall_summary 的第一句必须直接给结论（target_action + action_probability_percent）。
2) action_probability_percent 必须是具体数值。
3) 必须明确给出当前仓位占比、目标配置区间、差值。
4) 禁止“看情况/视情况/it depends”。`, symbolContext, string(frameworkIDsJSON), string(frameworkJSON), string(weightJSON))

	result, err := aiChatCompletion(ctx, aiChatCompletionRequest{
		EndpointURL:  endpoint,
		APIKey:       apiKey,
		Model:        model,
		SystemPrompt: symbolSynthesisSystemPrompt,
		UserPrompt:   userPrompt,
		OnDelta: func(delta string) {
			delta = strings.TrimSpace(delta)
			if delta == "" || onDelta == nil {
				return
			}
			onDelta("[synthesis] " + delta)
		},
	})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}
