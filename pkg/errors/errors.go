package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Domain errors — these never change, use them everywhere
var (
	ErrOrderNotFound       = errors.New("order not found")
	ErrOrderAlreadyFilled  = errors.New("order already filled")
	ErrOrderCancelled      = errors.New("order already cancelled")
	ErrInvalidSide         = errors.New("invalid order side")
	ErrInvalidPrice        = errors.New("price must be greater than zero")
	ErrInvalidQuantity     = errors.New("quantity must be greater than zero")
	ErrInvalidOrderType    = errors.New("invalid order type")
	ErrSymbolRequired      = errors.New("symbol is required")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
	ErrKafkaPublish        = errors.New("failed to publish to kafka")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrSelfTrade           = errors.New("self-trade not permitted")
)

// AppError wraps a domain error with an HTTP status code
// so API handlers can respond correctly without knowing business logic
type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("code=%d msg=%s err=%v", e.Code, e.Message, e.Err)
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// Map domain errors to HTTP responses — single place, no if/else scattered around
func ToHTTP(err error) *AppError {
	switch {
	case errors.Is(err, ErrOrderNotFound):
		return New(http.StatusNotFound, "Order not found", err)
	case errors.Is(err, ErrUnauthorized):
		return New(http.StatusUnauthorized, "Unauthorized", err)
	case errors.Is(err, ErrRateLimitExceeded):
		return New(http.StatusTooManyRequests, "Rate limit exceeded", err)
	case errors.Is(err, ErrOrderAlreadyFilled),
		errors.Is(err, ErrOrderCancelled),
		errors.Is(err, ErrInvalidSide),
		errors.Is(err, ErrInvalidPrice),
		errors.Is(err, ErrInvalidQuantity),
		errors.Is(err, ErrInvalidOrderType),
		errors.Is(err, ErrSymbolRequired),
		errors.Is(err, ErrSelfTrade):
		return New(http.StatusBadRequest, err.Error(), err)
	default:
		return New(http.StatusInternalServerError, "Internal server error", err)
	}
}
