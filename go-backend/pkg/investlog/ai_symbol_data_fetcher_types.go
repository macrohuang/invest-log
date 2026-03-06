package investlog

import (
	"time"
)

type externalDataSection struct {
	Source  string `json:"source"`
	Type    string `json:"type"` // "news", "financials", "research"
	Content string `json:"content"`
}

// symbolExternalData holds all fetched external data for a symbol.
type symbolExternalData struct {
	Symbol            string
	Market            string
	FetchedAt         time.Time
	RawSections       []externalDataSection
	Summary           string
	StructuredSummary string
}

// externalDataSource defines a single data source to fetch.
type externalDataSource struct {
	Name    string
	URL     string
	Headers map[string]string
	Parser  func(body []byte) (string, error)
}

const (
	externalDataFetchTimeout     = 30 * time.Second
	externalDataSummarizeTimeout = 30 * time.Second
	externalDataMaxChars         = 8000
)

type externalSummarySectionSpec struct {
	Header  string
	GapNote string
}

var externalSummarySectionSpecs = []externalSummarySectionSpec{
	{Header: "近5个季度财报", GapNote: "未抓取到近5个季度财报"},
	{Header: "近3年年报", GapNote: "未抓取到近3年年报"},
	{Header: "行业宏观政策", GapNote: "未抓取到行业宏观政策"},
	{Header: "产业周期", GapNote: "未抓取到产业周期信息"},
	{Header: "公司最新经营", GapNote: "未抓取到公司最新经营进展"},
}

// Function variables for testing/mocking.
var fetchExternalDataFn = fetchExternalDataImpl
var summarizeExternalDataFn = summarizeExternalDataImpl
