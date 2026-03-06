package investlog

import (
	"fmt"
	"strings"
)

func normalizeStructuredExternalSummary(summary string, data *symbolExternalData) string {
	normalized := strings.TrimSpace(summary)
	if normalized == "" {
		return buildFallbackStructuredExternalSummary(data)
	}

	missingSections := make([]string, 0)
	for _, spec := range externalSummarySectionSpecs {
		header := fmt.Sprintf("【%s】", spec.Header)
		if strings.Contains(normalized, header) {
			continue
		}
		normalized += fmt.Sprintf("\n\n%s\n- 缺口：%s", header, spec.GapNote)
		missingSections = append(missingSections, spec.Header)
	}

	if !strings.Contains(normalized, "【数据缺口】") {
		normalized += "\n\n【数据缺口】"
		if len(missingSections) == 0 {
			normalized += "\n- 无新增结构化缺口（仍需后续刷新）"
		} else {
			for _, section := range missingSections {
				normalized += fmt.Sprintf("\n- 缺口：%s数据不足", section)
			}
		}
	}

	return strings.TrimSpace(normalized)
}

func buildFallbackStructuredExternalSummary(data *symbolExternalData) string {
	if data == nil {
		return buildAllGapSummary()
	}
	lines := flattenExternalDataLines(data.RawSections)
	if len(lines) == 0 {
		return buildAllGapSummary()
	}

	quarterLines := pickEvidenceLines(lines, []string{"q1", "q2", "q3", "q4", "季度", "季报"}, 3)
	annualLines := pickEvidenceLines(lines, []string{"年报", "年度", "fy", "annual"}, 3)
	policyLines := pickEvidenceLines(lines, []string{"政策", "监管", "央行", "利率", "财政", "补贴", "关税"}, 3)
	cycleLines := pickEvidenceLines(lines, []string{"周期", "景气", "库存", "产能", "供需", "cycle"}, 3)
	operationLines := pickEvidenceLines(lines, []string{"营收", "净利润", "订单", "指引", "回购", "并购", "产线", "产品", "经营"}, 3)

	var builder strings.Builder
	missing := make([]string, 0)

	writeSummarySection(&builder, "近5个季度财报", quarterLines, "未抓取到近5个季度财报", &missing)
	writeSummarySection(&builder, "近3年年报", annualLines, "未抓取到近3年年报", &missing)
	writeSummarySection(&builder, "行业宏观政策", policyLines, "未抓取到行业宏观政策", &missing)
	writeSummarySection(&builder, "产业周期", cycleLines, "未抓取到产业周期信息", &missing)
	writeSummarySection(&builder, "公司最新经营", operationLines, "未抓取到公司最新经营进展", &missing)

	builder.WriteString("\n\n【数据缺口】")
	if len(missing) == 0 {
		builder.WriteString("\n- 无明确结构化缺口（建议继续刷新）")
	} else {
		for _, section := range missing {
			builder.WriteString(fmt.Sprintf("\n- 缺口：%s", section))
		}
	}
	return strings.TrimSpace(builder.String())
}

func writeSummarySection(builder *strings.Builder, header string, lines []string, gapNote string, missing *[]string) {
	if builder.Len() > 0 {
		builder.WriteString("\n\n")
	}
	builder.WriteString(fmt.Sprintf("【%s】", header))
	if len(lines) == 0 {
		builder.WriteString(fmt.Sprintf("\n- 缺口：%s", gapNote))
		*missing = append(*missing, header)
		return
	}
	for _, line := range lines {
		builder.WriteString("\n- ")
		builder.WriteString(line)
	}
}

func buildAllGapSummary() string {
	var builder strings.Builder
	for idx, spec := range externalSummarySectionSpecs {
		if idx > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(fmt.Sprintf("【%s】\n- 缺口：%s", spec.Header, spec.GapNote))
	}
	builder.WriteString("\n\n【数据缺口】")
	for _, spec := range externalSummarySectionSpecs {
		builder.WriteString(fmt.Sprintf("\n- 缺口：%s数据不足", spec.Header))
	}
	return builder.String()
}

func flattenExternalDataLines(sections []externalDataSection) []string {
	lines := make([]string, 0, len(sections)*3)
	for _, section := range sections {
		source := strings.TrimSpace(section.Source)
		for _, rawLine := range strings.Split(section.Content, "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}
			if len([]rune(line)) > 80 {
				line = string([]rune(line)[:80]) + "..."
			}
			if source != "" {
				line = fmt.Sprintf("%s: %s", source, line)
			}
			lines = append(lines, line)
			if len(lines) >= 120 {
				return lines
			}
		}
	}
	return lines
}

func pickEvidenceLines(lines []string, keywords []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	result := make([]string, 0, limit)
	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if len(keywords) > 0 && !containsAnyLowerKeyword(lowerLine, keywords) {
			continue
		}
		result = append(result, line)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func containsAnyLowerKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// buildDataSources returns the data sources for the given market.

func buildRawSectionsText(sections []externalDataSection) string {
	if len(sections) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, s := range sections {
		sb.WriteString(fmt.Sprintf("=== %s (%s) ===\n", s.Source, s.Type))
		sb.WriteString(s.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
