package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) CreateAlertChannel(ctx context.Context, projectID string, input map[string]interface{}) (*AlertChannel, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/projects/"+projectID+"/alert-channels", input)
	if err != nil {
		return nil, err
	}
	var channel AlertChannel
	if err := json.Unmarshal(body, &channel); err != nil {
		return nil, fmt.Errorf("decoding alert channel: %w", err)
	}
	return &channel, nil
}

func (c *Client) GetAlertChannel(ctx context.Context, projectID, channelID string) (*AlertChannel, error) {
	// API only has list endpoint — fetch all and filter
	body, _, err := c.doRequest(ctx, "GET", "/api/projects/"+projectID+"/alert-channels", nil)
	if err != nil {
		return nil, err
	}
	var resp AlertChannelListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding alert channels: %w", err)
	}
	for _, ch := range resp.Channels {
		if ch.ID == channelID {
			return &ch, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "alert channel not found"}
}

func (c *Client) UpdateAlertChannel(ctx context.Context, projectID, channelID string, input map[string]interface{}) (*AlertChannel, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/projects/"+projectID+"/alert-channels/"+channelID, input)
	if err != nil {
		return nil, err
	}
	var channel AlertChannel
	if err := json.Unmarshal(body, &channel); err != nil {
		return nil, fmt.Errorf("decoding alert channel: %w", err)
	}
	return &channel, nil
}

func (c *Client) DeleteAlertChannel(ctx context.Context, projectID, channelID string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/projects/"+projectID+"/alert-channels/"+channelID, nil)
	return err
}
