package investlog

import "fmt"

// ErrorCode defines error classification codes for structured error handling.
type ErrorCode string

// Error codes for different error categories.
const (
	ErrCodeInvalidInput     ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeDuplicate        ErrorCode = "DUPLICATE"
	ErrCodeInsufficientFund ErrorCode = "INSUFFICIENT_FUND"
	ErrCodeDatabase         ErrorCode = "DATABASE_ERROR"
	ErrCodeValidation       ErrorCode = "VALIDATION_ERROR"
	ErrCodeInternal         ErrorCode = "INTERNAL_ERROR"
	ErrCodeUnsupported      ErrorCode = "UNSUPPORTED"
)

// Error represents a structured error with classification code.
type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error for errors.Is and errors.As support.
func (e *Error) Unwrap() error {
	return e.Err
}

// NewError creates a new Error with the given code and message.
func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WrapError wraps an existing error with classification code and additional context.
func WrapError(code ErrorCode, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// IsErrorCode checks if an error matches a specific error code.
func IsErrorCode(err error, code ErrorCode) bool {
	var e *Error
	if ok := (err != nil && err.(*Error) != nil); ok {
		e = err.(*Error)
		return e.Code == code
	}
	return false
}
