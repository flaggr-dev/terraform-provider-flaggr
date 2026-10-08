package client

import (
	"encoding/json"
	"fmt"
)

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

// ChangeRequestError reports that Flaggr opened a change request instead of
// making a change (HTTP 202), because the environment requires approval.
// Nothing has changed yet: the change happens when someone approves and
// applies the request in Flaggr.
type ChangeRequestError struct {
	Environment string
	// ChangeRequestID is empty if the response didn't include the request.
	ChangeRequestID string
	// Message is the API's explanation, if it sent one.
	Message string
}

func (e *ChangeRequestError) Error() string {
	request := "a change request"
	if e.ChangeRequestID != "" {
		request = fmt.Sprintf("change request %s", e.ChangeRequestID)
	}
	return fmt.Sprintf("the %s environment requires approval: Flaggr opened %s instead of making the change", e.Environment, request)
}

func newChangeRequestError(environment string, body []byte) *ChangeRequestError {
	var accepted struct {
		Message       string `json:"message"`
		ChangeRequest struct {
			ID string `json:"id"`
		} `json:"changeRequest"`
	}
	_ = json.Unmarshal(body, &accepted)
	return &ChangeRequestError{
		Environment:     environment,
		ChangeRequestID: accepted.ChangeRequest.ID,
		Message:         accepted.Message,
	}
}
