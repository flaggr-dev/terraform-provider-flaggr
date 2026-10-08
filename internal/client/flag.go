package client

import (
	"context"
	"encoding/json"
	"fmt"
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

func (c *Client) GetFlag(ctx context.Context, key, serviceID, environment string) (*Flag, error) {
	params := url.Values{}
	params.Set("serviceId", serviceID)
	params.Set("environment", environment)

	body, _, err := c.doRequest(ctx, "GET", "/api/flags/"+key+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var flag Flag
	if err := json.Unmarshal(body, &flag); err != nil {
		return nil, fmt.Errorf("decoding flag: %w", err)
	}
	return &flag, nil
}

func (c *Client) UpdateFlag(ctx context.Context, key string, input map[string]interface{}) (*Flag, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/flags/"+key, input)
	if err != nil {
		return nil, err
	}
	var flag Flag
	if err := json.Unmarshal(body, &flag); err != nil {
		return nil, fmt.Errorf("decoding flag: %w", err)
	}
	return &flag, nil
}

func (c *Client) DeleteFlag(ctx context.Context, key, serviceID, environment string) error {
	params := url.Values{}
	params.Set("serviceId", serviceID)
	params.Set("environment", environment)

	_, _, err := c.doRequest(ctx, "DELETE", "/api/flags/"+key+"?"+params.Encode(), nil)
	return err
}
