package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) CreateFlag(ctx context.Context, input map[string]interface{}) (*Flag, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/flags", input)
	if err != nil {
		return nil, err
	}
	var flag Flag
	if err := json.Unmarshal(body, &flag); err != nil {
		return nil, fmt.Errorf("decoding flag: %w", err)
	}
	return &flag, nil
}

// flagPath is /api/flags/{key} with the flag's service and environment in the
// query string, which is where GET, PATCH and DELETE /api/flags/{key} look for
// them (the API defaults a missing environment to "development").
func flagPath(key, serviceID, environment string) string {
	params := url.Values{}
	params.Set("serviceId", serviceID)
	params.Set("environment", environment)
	return "/api/flags/" + key + "?" + params.Encode()
}

func (c *Client) GetFlag(ctx context.Context, key, serviceID, environment string) (*Flag, error) {
	body, _, err := c.doRequest(ctx, "GET", flagPath(key, serviceID, environment), nil)
	if err != nil {
		return nil, err
	}
	var flag Flag
	if err := json.Unmarshal(body, &flag); err != nil {
		return nil, fmt.Errorf("decoding flag: %w", err)
	}
	return &flag, nil
}

// UpdateFlag changes the flag with this key in one service and environment.
// In an environment that requires approval, Flaggr changes nothing: it opens
// a change request and answers HTTP 202, which UpdateFlag returns as a
// *ChangeRequestError.
func (c *Client) UpdateFlag(ctx context.Context, key, serviceID, environment string, input map[string]interface{}) (*Flag, error) {
	body, status, err := c.doRequest(ctx, "PATCH", flagPath(key, serviceID, environment), input)
	if err != nil {
		return nil, err
	}
	if status == http.StatusAccepted {
		return nil, newChangeRequestError(environment, body)
	}
	var flag Flag
	if err := json.Unmarshal(body, &flag); err != nil {
		return nil, fmt.Errorf("decoding flag: %w", err)
	}
	return &flag, nil
}

func (c *Client) DeleteFlag(ctx context.Context, key, serviceID, environment string) error {
	_, _, err := c.doRequest(ctx, "DELETE", flagPath(key, serviceID, environment), nil)
	return err
}
