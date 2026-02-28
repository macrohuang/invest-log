package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type loggingResponseWriter struct {
	middleware.WrapResponseWriter
	errorMessage string
}

func newLoggingResponseWriter(w http.ResponseWriter, r *http.Request) *loggingResponseWriter {
	return &loggingResponseWriter{WrapResponseWriter: middleware.NewWrapResponseWriter(w, r.ProtoMajor)}
}

func (w *loggingResponseWriter) SetErrorMessage(message string) {
	w.errorMessage = message
}

func (w *loggingResponseWriter) ErrorMessage() string {
	return w.errorMessage
}

func (w *loggingResponseWriter) Flush() {
	if flusher, ok := w.WrapResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func requestLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newLoggingResponseWriter(w, r)

			next.ServeHTTP(wrapped, r)

			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
			}

			fields := []any{
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"route", routePattern(r),
				"query", r.URL.RawQuery,
				"status", status,
				"bytes", wrapped.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_ip", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			}

			if message := wrapped.ErrorMessage(); message != "" {
				fields = append(fields, "error_message", message)
			}

			switch {
			case status >= http.StatusInternalServerError:
				logger.Error("http request completed", fields...)
			case status >= http.StatusBadRequest:
				logger.Warn("http request completed", fields...)
			default:
				logger.Info("http request completed", fields...)
			}
		})
	}
}

func recoveryLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered",
						"request_id", middleware.GetReqID(r.Context()),
						"method", r.Method,
						"path", r.URL.Path,
						"route", routePattern(r),
						"query", r.URL.RawQuery,
						"remote_ip", r.RemoteAddr,
						"user_agent", r.UserAgent(),
						"panic", fmt.Sprint(recovered),
						"stack", string(debug.Stack()),
					)

					if statusWriter, ok := w.(interface{ Status() int }); ok {
						if statusWriter.Status() != 0 {
							return
						}
					}
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}
	return rctx.RoutePattern()
}
