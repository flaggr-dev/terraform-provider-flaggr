package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) CreateMetricSource(ctx context.Context, projectID string, input map[string]interface{}) (*MetricSource, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/projects/"+projectID+"/metric-sources", input)
	if err != nil {
		return nil, err
	}
	var source MetricSource
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, fmt.Errorf("decoding metric source: %w", err)
	}
	return &source, nil
}

func (c *Client) GetMetricSource(ctx context.Context, projectID, sourceID string) (*MetricSource, error) {
	// API only has list endpoint — fetch all and filter
	body, _, err := c.doRequest(ctx, "GET", "/api/projects/"+projectID+"/metric-sources", nil)
	if err != nil {
		return nil, err
	}
	var resp MetricSourceListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding metric sources: %w", err)
	}
	for _, s := range resp.Sources {
		if s.ID == sourceID {
			return &s, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "metric source not found"}
}

func (c *Client) UpdateMetricSource(ctx context.Context, projectID, sourceID string, input map[string]interface{}) (*MetricSource, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/projects/"+projectID+"/metric-sources/"+sourceID, input)
	if err != nil {
		return nil, err
	}
	var source MetricSource
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, fmt.Errorf("decoding metric source: %w", err)
	}
	return &source, nil
}

func (c *Client) DeleteMetricSource(ctx context.Context, projectID, sourceID string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/projects/"+projectID+"/metric-sources/"+sourceID, nil)
	return err
}
