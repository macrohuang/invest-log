package investlog

import (
	"fmt"
	"strings"
)

func normalizeDimensionResult(result *SymbolDimensionResult, frameworkID string) {
	if result == nil {
		return
	}

	result.Dimension = strings.ToLower(strings.TrimSpace(result.Dimension))
	if strings.TrimSpace(frameworkID) != "" {
		result.Dimension = strings.ToLower(strings.TrimSpace(frameworkID))
	}
	result.Rating = strings.ToLower(strings.TrimSpace(result.Rating))
	if result.Rating == "" {
		result.Rating = "neutral"
	}
	result.Confidence = strings.ToLower(strings.TrimSpace(result.Confidence))
	if result.Confidence == "" {
		result.Confidence = "medium"
	}
	result.Summary = strings.TrimSpace(stripFuzzyPhrases(result.Summary))
	if result.Summary == "" {
		result.Summary = "数据不足，暂给中性判断。"
	}
	result.Suggestion = strings.TrimSpace(stripFuzzyPhrases(result.Suggestion))
	if result.Suggestion == "" {
		result.Suggestion = defaultDimensionSuggestion(result.Rating)
	}
}

func defaultDimensionSuggestion(rating string) string {
	switch strings.ToLower(strings.TrimSpace(rating)) {
	case "positive":
		return "increase：信号偏正，分批加仓。"
	case "negative":
		return "reduce：信号偏弱，先降仓。"
	default:
		return "hold：信号中性，维持仓位。"
	}
}

func normalizeSynthesisResult(result *SymbolSynthesisResult, context *symbolContextData, frameworkIDs ...[]string) {
	if result == nil {
		return
	}
	var selectedFrameworkIDs []string
	if len(frameworkIDs) > 0 {
		selectedFrameworkIDs = frameworkIDs[0]
	}
	result.TargetAction = normalizeSynthesisAction(result.TargetAction)
	result.ActionProbability = normalizeSynthesisProbability(result.Confidence, result.ActionProbability)
	result.PositionSuggestion = normalizeSynthesisPositionSuggestion(*result, context)
	result.OverallSummary = normalizeSynthesisSummary(*result, selectedFrameworkIDs)
	result.Disclaimer = normalizeSynthesisDisclaimer(result.Disclaimer)
}

func normalizeSynthesisAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "increase":
		return "increase"
	case "reduce":
		return "reduce"
	default:
		return "hold"
	}
}

func normalizeSynthesisProbability(confidence string, probability float64) float64 {
	if probability > 0 && probability <= 100 {
		return round2(probability)
	}

	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case "high":
		return 72
	case "low":
		return 42
	default:
		return 58
	}
}

func normalizeSynthesisSummary(result SymbolSynthesisResult, frameworkIDs ...[]string) string {
	var selectedFrameworkIDs []string
	if len(frameworkIDs) > 0 {
		selectedFrameworkIDs = frameworkIDs[0]
	}
	actionLabel := mapSynthesisActionLabel(result.TargetAction)
	probability := normalizeSynthesisProbability(result.Confidence, result.ActionProbability)

	position := compactSynthesisText(result.PositionSuggestion)
	if position == "" {
		position = "维持当前仓位。"
	} else if !strings.HasSuffix(position, "。") {
		position += "。"
	}

	factors := buildSynthesisListLine("依据", result.KeyFactors)
	risks := buildSynthesisListLine("雷点", result.RiskWarnings)

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("%s，执行概率%.0f%%。", actionLabel, probability))
	if len(selectedFrameworkIDs) > 0 {
		builder.WriteString("框架：")
		builder.WriteString(strings.Join(selectedFrameworkIDs, "、"))
		builder.WriteString("。")
	}
	builder.WriteString("仓位：")
	builder.WriteString(position)
	builder.WriteString(factors)
	builder.WriteString(risks)

	summary := strings.TrimSpace(stripFuzzyPhrases(builder.String()))

	if len([]rune(summary)) > 200 {
		summary = string([]rune(summary)[:200])
		if !strings.HasSuffix(summary, "。") {
			summary += "。"
		}
	}

	if summary == "" {
		summary = fmt.Sprintf("%s，执行概率%.0f%%。", actionLabel, probability)
	}

	return summary
}

func normalizeSynthesisDisclaimer(disclaimer string) string {
	cleaned := strings.TrimSpace(stripFuzzyPhrases(disclaimer))
	cleaned = strings.NewReplacer("\n", "", "\r", "", " ", "", "。", "", "，", "").Replace(cleaned)
	if cleaned == "" {
		cleaned = "市场波动风险"
	}
	runes := []rune(cleaned)
	if len(runes) > maxSynthesisDisclaimerChars {
		cleaned = string(runes[:maxSynthesisDisclaimerChars])
	}
	if strings.TrimSpace(cleaned) == "" {
		return "市场波动风险"
	}
	return cleaned
}

func stripFuzzyPhrases(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"看情况", "",
		"视情况", "",
		"视情况而定", "",
		"it depends", "",
		"It depends", "",
		"IT DEPENDS", "",
	)
	return replacer.Replace(input)
}

func normalizeSynthesisPositionSuggestion(result SymbolSynthesisResult, context *symbolContextData) string {
	base := compactSynthesisText(result.PositionSuggestion)
	if context == nil {
		if base == "" {
			return "当前占比未知；目标区间未知；差值未知。"
		}
		if strings.Contains(base, "当前占比") && strings.Contains(base, "目标区间") && strings.Contains(base, "差值") {
			if strings.HasSuffix(base, "。") {
				return base
			}
			return base + "。"
		}
		return base
	}

	current := round2(context.PositionPercent)
	targetMin, targetMax := context.AllocationMinPercent, context.AllocationMaxPercent
	if targetMin == 0 && targetMax == 0 {
		targetMin = 0
		targetMax = 100
	}

	delta := 0.0
	status := "在区间内"
	switch {
	case current < targetMin:
		delta = round2(current - targetMin)
		status = "低于下限"
	case current > targetMax:
		delta = round2(current - targetMax)
		status = "高于上限"
	default:
		delta = 0
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("当前占比%.2f%%；目标区间%.2f%%-%.2f%%；差值%s（%s）；动作：%s",
		current,
		targetMin,
		targetMax,
		formatSignedPercent(delta),
		status,
		mapSynthesisActionLabel(result.TargetAction),
	))

	if base != "" {
		builder.WriteString("；执行：")
		builder.WriteString(base)
	}

	position := builder.String()
	if !strings.HasSuffix(position, "。") {
		position += "。"
	}
	return position
}

func formatSignedPercent(value float64) string {
	if value > 0 {
		return fmt.Sprintf("+%.2f%%", value)
	}
	if value < 0 {
		return fmt.Sprintf("%.2f%%", value)
	}
	return "0.00%"
}

func buildSynthesisListLine(label string, items []string) string {
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		cleaned := compactSynthesisText(item)
		if cleaned != "" {
			normalized = append(normalized, cleaned)
		}
		if len(normalized) >= 2 {
			break
		}
	}
	if len(normalized) == 0 {
		return ""
	}
	return fmt.Sprintf("%s：%s。", label, strings.Join(normalized, "；"))
}

func compactSynthesisText(input string) string {
	trimmed := strings.TrimSpace(stripFuzzyPhrases(input))
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		"  ", " ",
		"。", "",
	)
	cleaned := replacer.Replace(trimmed)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}
	if len([]rune(cleaned)) > 42 {
		cleaned = string([]rune(cleaned)[:42])
	}
	return cleaned
}

func mapSynthesisActionLabel(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "increase":
		return "加仓"
	case "reduce":
		return "减仓"
	case "hold":
		return "持有"
	default:
		return "持有"
	}
}
