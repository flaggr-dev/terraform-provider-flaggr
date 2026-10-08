package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) CreateAlertRule(ctx context.Context, projectID string, input map[string]interface{}) (*AlertRule, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/projects/"+projectID+"/alerts", input)
	if err != nil {
		return nil, err
	}
	var rule AlertRule
	if err := json.Unmarshal(body, &rule); err != nil {
		return nil, fmt.Errorf("decoding alert rule: %w", err)
	}
	return &rule, nil
}

func (c *Client) GetAlertRule(ctx context.Context, projectID, ruleID string) (*AlertRule, error) {
	// API only has list endpoint — fetch all and filter
	body, _, err := c.doRequest(ctx, "GET", "/api/projects/"+projectID+"/alerts", nil)
	if err != nil {
		return nil, err
	}
	var resp AlertRuleListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding alert rules: %w", err)
	}
	for _, r := range resp.Rules {
		if r.ID == ruleID {
			return &r, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "alert rule not found"}
}

func (c *Client) UpdateAlertRule(ctx context.Context, projectID, ruleID string, input map[string]interface{}) (*AlertRule, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/projects/"+projectID+"/alerts/"+ruleID, input)
	if err != nil {
		return nil, err
	}
	var rule AlertRule
	if err := json.Unmarshal(body, &rule); err != nil {
		return nil, fmt.Errorf("decoding alert rule: %w", err)
	}
	return &rule, nil
}

func (c *Client) DeleteAlertRule(ctx context.Context, projectID, ruleID string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/projects/"+projectID+"/alerts/"+ruleID, nil)
	return err
}
