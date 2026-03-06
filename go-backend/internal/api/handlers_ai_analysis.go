package api

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"

	"investlog/pkg/investlog"
)

func (h *handler) getAIAnalysisMethods(w http.ResponseWriter, r *http.Request) {
	methods, err := h.core.ListAIAnalysisMethods()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, methods)
}

func (h *handler) createAIAnalysisMethod(w http.ResponseWriter, r *http.Request) {
	var payload aiAnalysisMethodPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	method, err := h.core.CreateAIAnalysisMethod(investlog.AIAnalysisMethod{
		Name:         payload.Name,
		SystemPrompt: payload.SystemPrompt,
		UserPrompt:   payload.UserPrompt,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, method)
}

func (h *handler) updateAIAnalysisMethod(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var payload aiAnalysisMethodPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	method, err := h.core.UpdateAIAnalysisMethod(id, investlog.AIAnalysisMethod{
		Name:         payload.Name,
		SystemPrompt: payload.SystemPrompt,
		UserPrompt:   payload.UserPrompt,
	})
	if err != nil {
		if err.Error() == "ai analysis method not found" {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, method)
}

func (h *handler) deleteAIAnalysisMethod(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	deleted, err := h.core.DeleteAIAnalysisMethod(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "ai analysis method not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (h *handler) runAIAnalysisStream(w http.ResponseWriter, r *http.Request) {
	var payload aiAnalysisStreamPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.MethodID <= 0 {
		writeError(w, http.StatusBadRequest, "method_id is required")
		return
	}

	method, err := h.core.GetAIAnalysisMethod(payload.MethodID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if method == nil {
		writeError(w, http.StatusNotFound, "ai analysis method not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	initSSEHeaders(w)
	w.WriteHeader(http.StatusOK)

	var streamMu sync.Mutex
	writeStreamEvent := func(event string, payload any) error {
		streamMu.Lock()
		defer streamMu.Unlock()
		return writeSSEEvent(w, flusher, event, payload)
	}

	if err := writeStreamEvent("progress", map[string]any{
		"stage":   "start",
		"message": "开始执行 AI Analysis",
	}); err != nil {
		h.logger.Warn("ai analysis stream write failed", "stage", "start", "err", err)
		return
	}

	result, err := h.core.RunAIAnalysisStream(investlog.RunAIAnalysisRequest{
		MethodID:  payload.MethodID,
		Variables: payload.Variables,
	}, func(delta string) error {
		if delta == "" {
			return nil
		}
		return writeStreamEvent("delta", map[string]string{"text": delta})
	})
	if err != nil {
		h.logger.Error("ai analysis stream failed", "method_id", payload.MethodID, "err", err)
		_ = writeStreamEvent("error", map[string]string{"error": err.Error()})
		_ = writeStreamEvent("done", map[string]any{"ok": false})
		return
	}

	_ = writeStreamEvent("result", result)
	_ = writeStreamEvent("done", map[string]any{"ok": true, "result": result})
}

func (h *handler) getAIAnalysisHistory(w http.ResponseWriter, r *http.Request) {
	methodID := parseInt64Default(r.URL.Query().Get("method_id"), 0)
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	runs, err := h.core.ListAIAnalysisRuns(methodID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (h *handler) getAIAnalysisRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	run, err := h.core.GetAIAnalysisRun(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "ai analysis run not found")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func parseInt64Default(raw string, fallback int64) int64 {
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}
