package main

import (
	"fmt"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// NetworkError represents network-related errors
	NetworkError ErrorType = "network_error"
	
	// ParseError represents parsing-related errors
	ParseError ErrorType = "parse_error"
	
	// ValidationError represents validation-related errors
	ValidationError ErrorType = "validation_error"
	
	// IOError represents I/O-related errors
	IOError ErrorType = "io_error"
	
	// ConfigError represents configuration-related errors
	ConfigError ErrorType = "config_error"
	
	// AuthError represents authentication-related errors
	AuthError ErrorType = "auth_error"
	
	// DownloadError represents download-related errors
	DownloadError ErrorType = "download_error"
)

// AppError represents a structured application error
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
	Code    string
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap implements the error unwrapping interface
func (e *AppError) Unwrap() error {
	return e.Err
}

// IsNetworkError checks if an error is a network error
func IsNetworkError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == NetworkError
	}
	return false
}

// IsParseError checks if an error is a parsing error
func IsParseError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ParseError
	}
	return false
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ValidationError
	}
	return false
}

// NewNetworkError creates a new network error
func NewNetworkError(message string, err error) *AppError {
	return &AppError{
		Type:    NetworkError,
		Message: message,
		Err:     err,
		Code:    "NET001",
	}
}

// NewParseError creates a new parsing error
func NewParseError(message string, err error) *AppError {
	return &AppError{
		Type:    ParseError,
		Message: message,
		Err:     err,
		Code:    "PARSE001",
	}
}

// NewValidationError creates a new validation error
func NewValidationError(message string) *AppError {
	return &AppError{
		Type:    ValidationError,
		Message: message,
		Code:    "VALID001",
	}
}

// NewIOError creates a new I/O error
func NewIOError(message string, err error) *AppError {
	return &AppError{
		Type:    IOError,
		Message: message,
		Err:     err,
		Code:    "IO001",
	}
}

// NewConfigError creates a new configuration error
func NewConfigError(message string) *AppError {
	return &AppError{
		Type:    ConfigError,
		Message: message,
		Code:    "CONFIG001",
	}
}

// NewAuthError creates a new authentication error
func NewAuthError(message string) *AppError {
	return &AppError{
		Type:    AuthError,
		Message: message,
		Code:    "AUTH001",
	}
}

// NewDownloadError creates a new download error
func NewDownloadError(message string, err error) *AppError {
	return &AppError{
		Type:    DownloadError,
		Message: message,
		Err:     err,
		Code:    "DL001",
	}
}