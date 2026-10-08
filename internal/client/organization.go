package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) CreateOrganization(ctx context.Context, input map[string]interface{}) (*Organization, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/organizations", input)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Organization Organization `json:"organization"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding organization: %w", err)
	}
	return &wrapper.Organization, nil
}

func (c *Client) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	body, _, err := c.doRequest(ctx, "GET", "/api/organizations/"+id, nil)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Organization Organization `json:"organization"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding organization: %w", err)
	}
	return &wrapper.Organization, nil
}

func (c *Client) GetOrganizationBySlug(ctx context.Context, slug string) (*Organization, error) {
	// List all orgs and find by slug (no dedicated slug endpoint)
	body, _, err := c.doRequest(ctx, "GET", "/api/organizations", nil)
	if err != nil {
		return nil, err
	}
	var resp OrganizationListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding organizations: %w", err)
	}
	for _, org := range resp.Organizations {
		if org.Slug == slug {
			return &org, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: fmt.Sprintf("organization with slug %q not found", slug)}
}

func (c *Client) UpdateOrganization(ctx context.Context, id string, input map[string]interface{}) (*Organization, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/organizations/"+id, input)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Organization Organization `json:"organization"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding organization: %w", err)
	}
	return &wrapper.Organization, nil
}

func (c *Client) DeleteOrganization(ctx context.Context, id string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/organizations/"+id, nil)
	return err
}

// Org Member operations

// orgMemberPageSize is the largest page GET /api/organizations/{id}/members
// serves (the API caps limit at 100 and defaults to 50).
const orgMemberPageSize = 100

// ListOrgMembers returns every member of the organization, following the
// API's limit/offset pages until it has `total` members. A response without
// `total` (a server that doesn't page) is complete as it stands.
func (c *Client) ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error) {
	var members []OrgMember
	seen := make(map[string]bool)
	offset := 0
	for {
		path := fmt.Sprintf("/api/organizations/%s/members?limit=%d&offset=%d", orgID, orgMemberPageSize, offset)
		body, _, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Members []OrgMember `json:"members"`
			Total   *int        `json:"total"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decoding org members: %w", err)
		}

		added := 0
		for _, member := range page.Members {
			// Members can shift between pages while we read; never list one twice.
			if member.ID != "" && seen[member.ID] {
				continue
			}
			seen[member.ID] = true
			members = append(members, member)
			added++
		}
		offset += len(page.Members)

		// Stop at the end, on an empty page, or when a page adds nothing new
		// (a server ignoring offset would otherwise repeat page one forever).
		if page.Total == nil || len(page.Members) == 0 || added == 0 || offset >= *page.Total {
			return members, nil
		}
	}
}

// AddOrgMember adds someone directly (input carries userId and role). The API
// answers an email with an invitation (202, no member) rather than adding the
// account; that is reported as an error, since no membership exists yet.
func (c *Client) AddOrgMember(ctx context.Context, orgID string, input map[string]interface{}) (*OrgMember, error) {
	body, _, err := c.doRequest(ctx, "POST", "/api/organizations/"+orgID+"/members", input)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Member  OrgMember `json:"member"`
		Invited bool      `json:"invited"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding org member: %w", err)
	}
	if wrapper.Invited || wrapper.Member.ID == "" {
		return nil, fmt.Errorf("the API sent an invitation instead of adding a member; add members by user ID")
	}
	return &wrapper.Member, nil
}

func (c *Client) UpdateOrgMember(ctx context.Context, orgID, memberID string, input map[string]interface{}) (*OrgMember, error) {
	body, _, err := c.doRequest(ctx, "PATCH", "/api/organizations/"+orgID+"/members/"+memberID, input)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Member OrgMember `json:"member"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decoding org member: %w", err)
	}
	return &wrapper.Member, nil
}

func (c *Client) RemoveOrgMember(ctx context.Context, orgID, memberID string) error {
	_, _, err := c.doRequest(ctx, "DELETE", "/api/organizations/"+orgID+"/members/"+memberID, nil)
	return err
}
