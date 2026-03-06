package investlog

import "net/http"

func setAIAuthHeader(httpReq *http.Request, endpoint, model, apiKey string) {
	if shouldUseGeminiAPI(endpoint, model) {
		httpReq.Header.Set("x-goog-api-key", apiKey)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
}
