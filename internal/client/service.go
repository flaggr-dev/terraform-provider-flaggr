package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) CreateService(ctx context.Context, input map[string]interface{}) (*Service, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/services", input)
	if err != nil {
		return nil, err
	}
	var service Service
	if err := json.Unmarshal(body, &service); err != nil {
		return nil, fmt.Errorf("decoding service: %w", err)
	}
	return &service, nil
}

func (c *Client) GetService(ctx context.Context, id string) (*Service, error) {
	body, _, err := c.doRequest(ctx, "GET", "/api/services/"+id, nil)
	if err != nil {
		return nil, err
	}
	var service Service
	if err := json.Unmarshal(body, &service); err != nil {
		return nil, fmt.Errorf("decoding service: %w", err)
	}
	return &service, nil
}

func (c *Client) UpdateService(ctx context.Context, id string, input map[string]interface{}) (*Service, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/services/"+id, input)
	if err != nil {
		return nil, err
	}
	var service Service
	if err := json.Unmarshal(body, &service); err != nil {
		return nil, fmt.Errorf("decoding service: %w", err)
	}
	return &service, nil
}

func (c *Client) DeleteService(ctx context.Context, id string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/services/"+id, nil)
	return err
}
