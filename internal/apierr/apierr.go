package apierr

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Error is the single error type returned by all handlers.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// New builds an API error.
func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func BadRequest(msg string) *Error   { return New(http.StatusBadRequest, "invalid_request", msg) }
func Unauthorized(msg string) *Error { return New(http.StatusUnauthorized, "unauthorized", msg) }
func Forbidden(msg string) *Error    { return New(http.StatusForbidden, "forbidden", msg) }
func NotFound(msg string) *Error     { return New(http.StatusNotFound, "not_found", msg) }
func Conflict(msg string) *Error     { return New(http.StatusConflict, "conflict", msg) }
func TooLarge(msg string) *Error     { return New(http.StatusRequestEntityTooLarge, "payload_too_large", msg) }

func RateLimited(msg string) *Error {
	return New(http.StatusTooManyRequests, "rate_limited", msg)
}

// Internal wraps an unknown error as a generic 500. The original error is
// never leaked to the client.
func Internal(err error) *Error {
	e := New(http.StatusInternalServerError, "internal", "خطای داخلی سرور")
	if err != nil {
		e.Details = nil
	}
	return e
}

// Write marshals the error; unknown errors become a generic 500.
func Write(w http.ResponseWriter, err error) {
	var ae *Error
	if !errors.As(err, &ae) {
		ae = Internal(err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(ae.Status)
	_ = json.NewEncoder(w).Encode(ae)
}

// WithDetails attaches structured details for the client.
func (e *Error) WithDetails(d any) *Error {
	e.Details = d
	return e
}
