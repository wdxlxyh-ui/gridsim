package errors

import (
	"encoding/json"
	"net/http"
	"strings"
)

type APIError struct {
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Hint       string   `json:"hint,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	Field      string   `json:"field,omitempty"`
}

func (e *APIError) Error() string { return e.Message }

type Response struct {
	Error APIError `json:"error"`
}

const (
	ErrBadRequest         = "BAD_REQUEST"
	ErrUnauthorized       = "UNAUTHORIZED"
	ErrNotFound           = "NOT_FOUND"
	ErrConflict           = "CONFLICT"
	ErrMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	ErrInternal           = "INTERNAL_ERROR"
	ErrInstanceNotFound   = "INSTANCE_NOT_FOUND"
	ErrInstanceNotRunning = "INSTANCE_NOT_RUNNING"
	ErrPortInUse          = "PORT_IN_USE"
	ErrIOANotFound        = "IOA_NOT_FOUND"
	ErrInvalidJSON        = "INVALID_JSON"
	ErrFileNotFound       = "FILE_NOT_FOUND"
	ErrFileTypeNotAllowed = "FILE_TYPE_NOT_ALLOWED"
)

func Respond(w http.ResponseWriter, status int, apiErr APIError) {
	if apiErr.Code == "" {
		apiErr.Code = codeFromStatus(status)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{Error: apiErr})
}

func RespondSimple(w http.ResponseWriter, status int, msg string) {
	Respond(w, status, APIError{
		Code:    codeFromStatus(status),
		Message: msg,
	})
}

func codeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusMethodNotAllowed:
		return ErrMethodNotAllowed
	case http.StatusInternalServerError:
		return ErrInternal
	default:
		return "ERROR"
	}
}

func CodeFromMessage(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "not found") || strings.Contains(lower, "不存在"):
		return ErrNotFound
	case strings.Contains(lower, "already") || strings.Contains(lower, "conflict") || strings.Contains(lower, "已存在") || strings.Contains(lower, "已占用"):
		return ErrConflict
	case strings.Contains(lower, "invalid json") || strings.Contains(lower, "json"):
		return ErrInvalidJSON
	case strings.Contains(lower, "not running") || strings.Contains(lower, "未运行"):
		return ErrInstanceNotRunning
	case strings.Contains(lower, "method not allowed"):
		return ErrMethodNotAllowed
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "credentials"):
		return ErrUnauthorized
	default:
		return ErrBadRequest
	}
}
