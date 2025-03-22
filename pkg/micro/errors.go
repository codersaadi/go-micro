package micro

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

// APIError represents an API error
type APIError struct {
	Code      int               `json:"code"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %d - %s", e.Code, e.Message)
}

// NewAPIError creates a new API error with optionaNewAPIErrorl details
func NewAPIError(code int, message string, details ...map[string]string) *APIError {
	err := &APIError{
		Code:    code,
		Message: message,
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

var (
	ErrInternalServer = NewAPIError(500, "internal server error")
)

// Enhanced error handling
func (a *App) handleError(w http.ResponseWriter, err error) {
	reqID := getRequestIDFromContext(w)
	apiError := a.normalizeError(err, reqID)

	a.Logger.Error("request error",
		zap.Error(err),
		zap.String("request_id", reqID),
		zap.Int("status_code", apiError.Code),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiError.Code)
	json.NewEncoder(w).Encode(apiError)
}

func (a *App) normalizeError(err error, requestID string) *APIError {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		apiErr = NewAPIError(http.StatusInternalServerError, "internal server error")
	}

	apiErr.RequestID = requestID
	if a.Config.LogLevel != "debug" {
		apiErr.Details = nil // Remove details in production
	}
	return apiErr
}
