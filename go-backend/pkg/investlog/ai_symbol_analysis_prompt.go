package investlog

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildDimensionUserPrompt constructs the user prompt for framework agents,
// optionally injecting enriched context from external data.
func buildDimensionUserPrompt(symbolContext, enrichedContext string, req SymbolAnalysisRequest, selectedFrameworkIDs []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请分析以下投资标的：\n%s\n", symbolContext))

	if len(selectedFrameworkIDs) > 0 {
		sb.WriteString(fmt.Sprintf("\n本次只允许分析以下框架ID：%s\n", strings.Join(selectedFrameworkIDs, ", ")))
	}

	if enrichedContext != "" {
		sb.WriteString(fmt.Sprintf(`
以下是该标的的最新实时数据和新闻摘要（数据截至今日）：
%s

重要：请优先使用上述实时数据进行分析，而非你的训练数据中的过时信息。
`, enrichedContext))
	}

	sb.WriteString("\n请根据你的框架职责进行分析，并输出指定格式的 JSON 结果。")

	if req.RiskProfile != "" || req.Horizon != "" || req.AdviceStyle != "" || req.StrategyPrompt != "" {
		preferenceJSON, err := json.Marshal(symbolPreferenceContext{
			RiskProfile:    req.RiskProfile,
			Horizon:        req.Horizon,
			AdviceStyle:    req.AdviceStyle,
			StrategyPrompt: req.StrategyPrompt,
		})
		if err == nil {
			sb.WriteString(fmt.Sprintf("\n\n用户投资偏好（用于结论权重与表达强度）：\n%s", string(preferenceJSON)))
		}
	}

	return sb.String()
}
