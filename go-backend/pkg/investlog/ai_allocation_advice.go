package investlog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const allocationAdviceSystemPrompt = `你是一个专业的资产配置顾问，精通现代投资组合理论，擅长将学术理论转化为实际可执行的配置建议。

你必须综合应用以下理论框架：
1) 马尔基尔（Malkiel）随机漫步理论：市场有效，长期持有低成本分散化指数组合优于主动选股；建议通过资产类别分散而非个股集中来获取市场回报。
2) 生命周期投资理论：随年龄增长应逐步降低高风险资产（股票）比例，增加稳健资产（债券、现金）比例；年轻投资者可承受更高波动换取更高长期回报。
3) 现代投资组合理论（Markowitz）：通过相关性低的资产组合降低整体风险；分散化是唯一的"免费午餐"。
4) 资产配置决定论：学术研究表明约90%的长期收益差异由资产配置决定，而非选股或择时。

你的任务是根据用户提供的个人信息和投资偏好，为其持有的各类资产在各币种下给出合理的最低/最高占比区间建议。

输出要求：
- 必须输出纯 JSON 对象，不要输出 Markdown 代码块，不要有任何额外文字
- JSON 字段：
  - summary: string（整体配置思路的简要说明，2-3句话）
  - rationale: string（基于用户画像的配置逻辑，说明为何如此建议）
  - allocations: [{currency, asset_type, label, min_percent, max_percent, rationale}]（每条针对一个资产类型+币种组合）
  - disclaimer: string（风险提示）
- allocations 必须覆盖输入中提供的所有 asset_types × currencies 组合
- min_percent 和 max_percent 必须是 0-100 之间的数字，且 min_percent <= max_percent
- 同一 currency 下所有 min_percent 之和不应超过 100，max_percent 之和不应低于 100
- rationale 字段说明该资产类型在该币种下配置此区间的具体理由（1-2句话）
- 禁止承诺收益，必须体现风险提示
- 如用户未提供某些信息，基于最稳健的假设给出建议`

// AllocationAdviceRequest defines the inputs for AI allocation advice.
type AllocationAdviceRequest struct {
	BaseURL         string
	APIKey          string
	Model           string
	AgeRange        string // "20s", "30s", "40s", "50s", "60plus"
	InvestGoal      string // "preserve", "income", "growth", "balanced"
	RiskTolerance   string // "conservative", "balanced", "aggressive"
	Horizon         string // "short", "medium", "long"
	ExperienceLevel string // "beginner", "intermediate", "experienced"
	Currencies      []string
	CustomPrompt    string
}

// AllocationAdviceEntry is one recommended allocation band for a currency+asset_type pair.
type AllocationAdviceEntry struct {
	Currency   string  `json:"currency"`
	AssetType  string  `json:"asset_type"`
	Label      string  `json:"label"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
	Rationale  string  `json:"rationale"`
}

// AllocationAdviceResult is the structured response returned to clients.
type AllocationAdviceResult struct {
	GeneratedAt string                  `json:"generated_at"`
	Model       string                  `json:"model"`
	Summary     string                  `json:"summary"`
	Rationale   string                  `json:"rationale"`
	Allocations []AllocationAdviceEntry `json:"allocations"`
	Disclaimer  string                  `json:"disclaimer"`
}

type allocationAdviceModelResponse struct {
	Summary     string                       `json:"summary"`
	Rationale   string                       `json:"rationale"`
	Allocations []allocationAdviceEntryModel `json:"allocations"`
	Disclaimer  string                       `json:"disclaimer"`
}

type allocationAdviceEntryModel struct {
	Currency   string  `json:"currency"`
	AssetType  string  `json:"asset_type"`
	Label      string  `json:"label"`
	MinPercent float64 `json:"min_percent"`
	MaxPercent float64 `json:"max_percent"`
	Rationale  string  `json:"rationale"`
}

type allocationAdvicePromptInput struct {
	AgeRange        string   `json:"age_range"`
	InvestGoal      string   `json:"invest_goal"`
	RiskTolerance   string   `json:"risk_tolerance"`
	Horizon         string   `json:"horizon"`
	ExperienceLevel string   `json:"experience_level"`
	Currencies      []string `json:"currencies"`
	AssetTypes      []struct {
		Code  string `json:"code"`
		Label string `json:"label"`
	} `json:"asset_types"`
	CustomPrompt string `json:"custom_prompt,omitempty"`
}

// GetAllocationAdvice generates AI-powered asset allocation recommendations.
func (c *Core) GetAllocationAdvice(req AllocationAdviceRequest) (*AllocationAdviceResult, error) {
	return c.getAllocationAdvice(req, nil)
}

// GetAllocationAdviceWithStream generates AI allocation advice and emits model delta chunks.
func (c *Core) GetAllocationAdviceWithStream(req AllocationAdviceRequest, onDelta func(string)) (*AllocationAdviceResult, error) {
	return c.getAllocationAdvice(req, onDelta)
}

func (c *Core) getAllocationAdvice(req AllocationAdviceRequest, onDelta func(string)) (*AllocationAdviceResult, error) {
	if err := normalizeAllocationAdviceRequest(&req); err != nil {
		return nil, err
	}

	assetTypes, err := c.GetAssetTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch asset types: %w", err)
	}
	if len(assetTypes) == 0 {
		return nil, fmt.Errorf("no asset types configured; add asset types before requesting AI advice")
	}

	userPrompt, err := buildAllocationAdviceUserPrompt(req, assetTypes)
	if err != nil {
		return nil, err
	}

	endpointURL, err := buildAICompletionsEndpoint(req.BaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), aiTotalRequestTimeout)
	defer cancel()

	chatResult, err := aiChatCompletion(ctx, aiChatCompletionRequest{
		EndpointURL:  endpointURL,
		APIKey:       req.APIKey,
		Model:        req.Model,
		SystemPrompt: allocationAdviceSystemPrompt,
		UserPrompt:   userPrompt,
		Logger:       c.Logger(),
		OnDelta:      onDelta,
	})
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	parsed, err := parseAllocationAdviceResponse(chatResult.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	allocations := make([]AllocationAdviceEntry, 0, len(parsed.Allocations))
	for _, a := range parsed.Allocations {
		entry := AllocationAdviceEntry{
			Currency:   strings.ToUpper(a.Currency),
			AssetType:  a.AssetType,
			Label:      a.Label,
			MinPercent: clampPercent(a.MinPercent),
			MaxPercent: clampPercent(a.MaxPercent),
			Rationale:  a.Rationale,
		}
		if entry.MinPercent > entry.MaxPercent {
			entry.MinPercent, entry.MaxPercent = entry.MaxPercent, entry.MinPercent
		}
		allocations = append(allocations, entry)
	}

	return &AllocationAdviceResult{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Model:       chatResult.Model,
		Summary:     parsed.Summary,
		Rationale:   parsed.Rationale,
		Allocations: allocations,
		Disclaimer:  parsed.Disclaimer,
	}, nil
}

func normalizeAllocationAdviceRequest(req *AllocationAdviceRequest) error {
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.Model = strings.TrimSpace(req.Model)

	if req.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	if req.BaseURL == "" {
		req.BaseURL = defaultAIBaseURL
	}

	if len(req.Currencies) == 0 {
		req.Currencies = []string{"CNY", "USD", "HKD"}
	}
	currencies := make([]string, 0, len(req.Currencies))
	for _, c := range req.Currencies {
		upper := strings.ToUpper(strings.TrimSpace(c))
		if upper == "CNY" || upper == "USD" || upper == "HKD" {
			currencies = append(currencies, upper)
		}
	}
	if len(currencies) == 0 {
		return fmt.Errorf("at least one valid currency (CNY, USD, HKD) is required")
	}
	req.Currencies = currencies

	return nil
}

func buildAllocationAdviceUserPrompt(req AllocationAdviceRequest, assetTypes []AssetType) (string, error) {
	input := allocationAdvicePromptInput{
		AgeRange:        req.AgeRange,
		InvestGoal:      req.InvestGoal,
		RiskTolerance:   req.RiskTolerance,
		Horizon:         req.Horizon,
		ExperienceLevel: req.ExperienceLevel,
		Currencies:      req.Currencies,
		CustomPrompt:    req.CustomPrompt,
	}
	for _, at := range assetTypes {
		input.AssetTypes = append(input.AssetTypes, struct {
			Code  string `json:"code"`
			Label string `json:"label"`
		}{Code: at.Code, Label: at.Label})
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to serialize prompt input: %w", err)
	}

	prompt := fmt.Sprintf(`请根据以下用户信息和资产类别，给出各资产在各币种下的最低/最高配置比例建议：

%s

字段说明：
- age_range: 用户年龄段（20s=20-29岁, 30s=30-39岁, 40s=40-49岁, 50s=50-59岁, 60plus=60岁以上）
- invest_goal: 投资目标（preserve=保值, income=稳定收益, growth=资本增值, balanced=均衡）
- risk_tolerance: 风险承受能力（conservative=保守, balanced=均衡, aggressive=激进）
- horizon: 投资期限（short=1-3年, medium=3-10年, long=10年以上）
- experience_level: 投资经验（beginner=新手, intermediate=有一定经验, experienced=丰富经验）
- currencies: 需要生成建议的币种列表
- asset_types: 用户当前已配置的资产类别（必须覆盖所有 currencies × asset_types 组合）

输出要求：
1) 必须是纯 JSON 对象，无任何额外文字或 Markdown 标记
2) allocations 数组必须包含所有 currencies × asset_types 的组合（共 %d 个组合）
3) 同一 currency 下的 min_percent 之和不超过 100，max_percent 之和不低于 100
4) 每个 allocation 的 rationale 需体现该资产在用户画像下的配置逻辑`, string(payload), len(req.Currencies)*len(assetTypes))

	return prompt, nil
}

func parseAllocationAdviceResponse(content string) (*allocationAdviceModelResponse, error) {
	cleaned := cleanupModelJSON(content)
	var parsed allocationAdviceModelResponse
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	return &parsed, nil
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
