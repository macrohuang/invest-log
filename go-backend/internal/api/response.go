package api

import (
	"net/http"

	"investlog/pkg/investlog"
)

// Response represents a successful API response with unified format.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse represents an error API response with structured information.
type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// writeSuccess writes a successful response with data.
func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Code: 0,
		Data: data,
	})
}

// writeSuccessWithMessage writes a successful response with data and message.
func writeSuccessWithMessage(w http.ResponseWriter, message string, data interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// writeErrorResponse writes an error response with proper HTTP status and error details.
func writeErrorResponse(w http.ResponseWriter, httpStatus int, err error) {
	response := ErrorResponse{
		Code:    httpStatus,
		Message: err.Error(),
	}

	// Extract error code if it's a structured error
	if invErr, ok := err.(*investlog.Error); ok {
		response.ErrorCode = string(invErr.Code)
		// Map business error codes to appropriate HTTP status codes
		httpStatus = mapErrorCodeToHTTPStatus(invErr.Code)
		response.Code = httpStatus
	}

	// TODO: Add request ID from context when middleware is implemented
	// requestID := r.Context().Value("request_id")

	writeJSON(w, httpStatus, response)
}

// mapErrorCodeToHTTPStatus maps business error codes to HTTP status codes.
func mapErrorCodeToHTTPStatus(code investlog.ErrorCode) int {
	switch code {
	case investlog.ErrCodeInvalidInput, investlog.ErrCodeValidation:
		return http.StatusBadRequest
	case investlog.ErrCodeNotFound:
		return http.StatusNotFound
	case investlog.ErrCodeDuplicate:
		return http.StatusConflict
	case investlog.ErrCodeInsufficientFund:
		return http.StatusBadRequest
	case investlog.ErrCodeDatabase, investlog.ErrCodeInternal:
		return http.StatusInternalServerError
	case investlog.ErrCodeUnsupported:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
