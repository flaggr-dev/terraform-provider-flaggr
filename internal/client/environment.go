package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) ListEnvironments(ctx context.Context, projectID string) ([]Environment, error) {
	body, _, err := c.doRequest(ctx, "GET", "/api/projects/"+projectID+"/environments", nil)
	if err != nil {
		return nil, err
	}
	var resp EnvironmentListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding environments: %w", err)
	}
	return resp.Environments, nil
}

func (c *Client) GetEnvironment(ctx context.Context, projectID, slug string) (*Environment, error) {
	// API only has list endpoint — fetch all and filter by slug
	envs, err := c.ListEnvironments(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		if env.Slug == slug {
			return &env, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "environment not found"}
}

func (c *Client) CreateEnvironment(ctx context.Context, projectID string, input map[string]interface{}) (*Environment, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/projects/"+projectID+"/environments", input)
	if err != nil {
		return nil, err
	}
	var env Environment
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decoding environment: %w", err)
	}
	return &env, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, projectID, slug string, input map[string]interface{}) (*Environment, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/projects/"+projectID+"/environments/"+slug, input)
	if err != nil {
		return nil, err
	}
	var env Environment
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decoding environment: %w", err)
	}
	return &env, nil
}

func (c *Client) DeleteEnvironment(ctx context.Context, projectID, slug string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/projects/"+projectID+"/environments/"+slug, nil)
	return err
}
