package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"investlog/pkg/investlog"
)

func TestWriteSuccess(t *testing.T) {
	rr := httptest.NewRecorder()
	writeSuccess(rr, map[string]string{"ok": "yes"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok || data["ok"] != "yes" {
		t.Fatalf("unexpected data payload: %v", resp.Data)
	}
}

func TestWriteSuccessWithMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	writeSuccessWithMessage(rr, "done", map[string]string{"status": "ok"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp Response
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Message != "done" {
		t.Fatalf("expected message %q, got %q", "done", resp.Message)
	}
}

func TestWriteErrorResponse(t *testing.T) {
	t.Run("structured error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		writeErrorResponse(rr, http.StatusInternalServerError, investlog.NewError(investlog.ErrCodeNotFound, "missing"))

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rr.Code)
		}
		var resp ErrorResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.ErrorCode != string(investlog.ErrCodeNotFound) {
			t.Fatalf("expected error_code %q, got %q", investlog.ErrCodeNotFound, resp.ErrorCode)
		}
	})

	t.Run("plain error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		writeErrorResponse(rr, http.StatusBadRequest, errors.New("bad input"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rr.Code)
		}
	})
}

func TestMapErrorCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		code investlog.ErrorCode
		want int
	}{
		{name: "invalid", code: investlog.ErrCodeInvalidInput, want: http.StatusBadRequest},
		{name: "validation", code: investlog.ErrCodeValidation, want: http.StatusBadRequest},
		{name: "not found", code: investlog.ErrCodeNotFound, want: http.StatusNotFound},
		{name: "duplicate", code: investlog.ErrCodeDuplicate, want: http.StatusConflict},
		{name: "insufficient", code: investlog.ErrCodeInsufficientFund, want: http.StatusBadRequest},
		{name: "database", code: investlog.ErrCodeDatabase, want: http.StatusInternalServerError},
		{name: "internal", code: investlog.ErrCodeInternal, want: http.StatusInternalServerError},
		{name: "unsupported", code: investlog.ErrCodeUnsupported, want: http.StatusNotImplemented},
		{name: "default", code: investlog.ErrorCode("UNKNOWN"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapErrorCodeToHTTPStatus(tt.code)
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}
