package client

import "fmt"

// APIError represents an error response from the Flaggr API.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("flaggr API error (HTTP %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("flaggr API error (HTTP %d): %s", e.StatusCode, e.Body)
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsNotFoundError checks if an error is a 404 API error.
func IsNotFoundError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsNotFound()
	}
	return false
}
