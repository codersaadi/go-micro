package micro

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

// loggingResponseWriter needs to include context in its struct
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	context    context.Context
}

func (a *App) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := xid.New().String()
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), contextKeyRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			context:        r.Context(),
		}

		next.ServeHTTP(lrw, r)

		a.Logger.Info("request processed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Int("status", lrw.statusCode),
			zap.Duration("duration", time.Since(start)),
			zap.String("request_id", lrw.context.Value(contextKeyRequestID).(string)),
		)
	})
}

func (a *App) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := r.Context().Value(contextKeyRequestID).(string)
				a.Logger.Error("panic recovered",
					zap.Any("error", err),
					zap.String("request_id", requestID),
				)
				a.handleError(w, NewAPIError(http.StatusInternalServerError, "Internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *App) timeoutMiddleware(timeout time.Duration) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *App) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			context:        r.Context(),
		}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(lrw.statusCode)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		httpDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

func (a *App) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomain")
		next.ServeHTTP(w, r)
	})
}

// Add context key type for type safety
type contextKey string

const (
	contextKeyRequestID contextKey = "request_id"
)
