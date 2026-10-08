package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) CreateProject(ctx context.Context, input map[string]interface{}) (*Project, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/projects", input)
	if err != nil {
		return nil, err
	}
	return decodeProject(body)
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	body, _, err := c.doRequest(ctx, "GET", "/api/projects/"+id, nil)
	if err != nil {
		return nil, err
	}
	return decodeProject(body)
}

func (c *Client) GetProjectBySlug(ctx context.Context, slug string) (*Project, error) {
	body, _, err := c.doRequest(ctx, "GET", "/api/projects/by-slug/"+slug, nil)
	if err != nil {
		return nil, err
	}
	return decodeProject(body)
}

func (c *Client) UpdateProject(ctx context.Context, id string, input map[string]interface{}) (*Project, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/projects/"+id, input)
	if err != nil {
		return nil, err
	}
	return decodeProject(body)
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/projects/"+id, nil)
	return err
}

// decodeProject reads the project out of a response body. POST /api/projects,
// GET and PATCH /api/projects/{id} and GET /api/projects/by-slug/{slug} answer
// {"project": {...}}; a bare project object is accepted too. A response
// without a project ID is an error: state without an ID can't be read,
// updated or deleted again.
func decodeProject(body []byte) (*Project, error) {
	var wrapped struct {
		Project *Project `json:"project"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("decoding project: %w", err)
	}
	project := wrapped.Project
	if project == nil {
		project = &Project{}
		if err := json.Unmarshal(body, project); err != nil {
			return nil, fmt.Errorf("decoding project: %w", err)
		}
	}
	if project.ID == "" {
		return nil, fmt.Errorf("decoding project: the response has no project ID")
	}
	return project, nil
}
