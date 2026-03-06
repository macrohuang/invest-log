const state = {
  apiBase: '',
  privacy: false,
  aiAnalysisByCurrency: {},
  aiAnalysisHistoryByCurrency: {}, // { currency: HoldingsAnalysisResult[] }
  aiStreamingByCurrency: {}, // { currency: { active, stage, text, content, error } }
  holdingsFilters: {}, // { currency: { accountIds: [], symbols: [] } }
  aiSettings: null,
  aiSettingsLoaded: false,
  aiAnalysisMethods: [],
  aiAnalysisMethodsLoaded: false,
  aiAnalysisSelectedMethodId: 0,
  aiAnalysisSelectedRunId: 0,
  aiAnalysisDraftValuesByMethod: {},
  aiAnalysisStreaming: null,
};

// Tracks which filter popover is open across re-renders: { filterType, currency } | null
let _openPopover = null;

const aiAnalysisSettingsKey = 'aiHoldingsAnalysisSettings';
const aiAnalysisAPIKeyStorageKey = 'aiHoldingsAnalysisApiKey';
const legacyOpenAIBaseURL = 'https://api.openai.com/v1';
const legacyGoogleGeminiBaseURL = 'https://generativelanguage.googleapis.com/v1beta';
const defaultGeminiBaseURL = 'https://api.aicodemirror.com/api/gemini';

const view = document.getElementById('view');
const toastEl = document.getElementById('toast');
const connectionPill = document.getElementById('connection-pill');
const privacyToggle = document.getElementById('privacy-toggle');
const navLinks = Array.from(document.querySelectorAll('.nav a'));

const currencySymbols = {
  CNY: '¥',
  USD: '$',
  HKD: 'HK$'
};

const chartPalette = [
  '#f06c3b',
  '#1aa6b7',
  '#f2a93b',
  '#6b7f66',
  '#c44b22',
  '#5b8fb9',
  '#b76a58',
  '#7a9a4e',
  '#8a6bd4',
  '#3f463e'
];
