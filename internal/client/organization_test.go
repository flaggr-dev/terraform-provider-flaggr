package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// memberServer serves `count` org members the way the Flaggr API does:
// limit (default 50, max 100) and offset, plus the total.
func memberServer(t *testing.T, count int, requests *[]string) *httptest.Server {
	t.Helper()
	all := make([]OrgMember, count)
	for i := range all {
		all[i] = OrgMember{ID: fmt.Sprintf("m-%03d", i), OrgID: "org-1", UserID: fmt.Sprintf("u-%03d", i), Role: "member"}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.URL.RequestURI())
		if r.URL.Path != "/api/organizations/org-1/members" {
			http.NotFound(w, r)
			return
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil {
			limit = 50
		}
		if limit > 100 {
			limit = 100
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		end := offset + limit
		if end > len(all) {
			end = len(all)
		}
		page := []OrgMember{}
		if offset < len(all) {
			page = all[offset:end]
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"members": page, "total": len(all), "limit": limit, "offset": offset})
	}))
}

func TestListOrgMembersReadsEveryPage(t *testing.T) {
	var requests []string
	server := memberServer(t, 250, &requests)
	defer server.Close()

	members, err := NewClient(server.URL, "fgp_test").ListOrgMembers(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ListOrgMembers: %v", err)
	}
	if len(members) != 250 {
		t.Fatalf("got %d members, want 250", len(members))
	}
	if members[0].ID != "m-000" || members[249].ID != "m-249" {
		t.Fatalf("members out of order: first %s, last %s", members[0].ID, members[249].ID)
	}
	want := []string{
		"/api/organizations/org-1/members?limit=100&offset=0",
		"/api/organizations/org-1/members?limit=100&offset=100",
		"/api/organizations/org-1/members?limit=100&offset=200",
	}
	if fmt.Sprint(requests) != fmt.Sprint(want) {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
}

func TestListOrgMembersStopsOnAnExactLastPage(t *testing.T) {
	var requests []string
	server := memberServer(t, 100, &requests)
	defer server.Close()

	members, err := NewClient(server.URL, "fgp_test").ListOrgMembers(context.Background(), "org-1")
	if err != nil || len(members) != 100 {
		t.Fatalf("members = %d, err = %v", len(members), err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %v, want one", requests)
	}
}

func TestListOrgMembersAcceptsAnUnpagedServer(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"members":[{"id":"m-1"},{"id":"m-2"}]}`))
	}))
	defer server.Close()

	members, err := NewClient(server.URL, "fgp_test").ListOrgMembers(context.Background(), "org-1")
	if err != nil || len(members) != 2 || calls != 1 {
		t.Fatalf("members = %v, err = %v, calls = %d", members, err, calls)
	}
}

func TestListOrgMembersNeverLoopsOnAServerIgnoringOffset(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Always page one, with a total that promises more.
		_, _ = w.Write([]byte(`{"members":[{"id":"m-1"},{"id":"m-2"}],"total":500}`))
	}))
	defer server.Close()

	members, err := NewClient(server.URL, "fgp_test").ListOrgMembers(context.Background(), "org-1")
	if err != nil || len(members) != 2 {
		t.Fatalf("members = %v, err = %v", members, err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (stop once a page adds nothing)", calls)
	}
}

func TestListOrgMembersReturnsAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Forbidden: project API tokens can't use organization endpoints"}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "fgr_test").ListOrgMembers(context.Background(), "org-1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("err = %v, want a 403 APIError", err)
	}
}

func TestAddOrgMemberPostsTheUserIDAndReadsTheMember(t *testing.T) {
	var got map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/organizations/org-1/members" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"member":{"id":"om-1","orgId":"org-1","userId":"u-1","role":"member","user":{"id":"u-1","email":"ann@acme.com"}}}`))
	}))
	defer server.Close()

	member, err := NewClient(server.URL, "fgp_test").AddOrgMember(context.Background(), "org-1", map[string]interface{}{"userId": "u-1", "role": "member"})
	if err != nil {
		t.Fatalf("AddOrgMember: %v", err)
	}
	if got["userId"] != "u-1" || got["role"] != "member" || got["email"] != nil {
		t.Fatalf("request body = %#v", got)
	}
	if member.ID != "om-1" || member.UserID != "u-1" || member.User == nil || member.User.Email != "ann@acme.com" {
		t.Fatalf("member = %#v", member)
	}
}

func TestAddOrgMemberRefusesAnInvitationAnswer(t *testing.T) {
	// The API answers an address with an emailed invitation, never a membership.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"invited":true,"invitation":{"id":"oinv-1","orgId":"org-1","email":"ann@acme.com","role":"member"}}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "fgp_test").AddOrgMember(context.Background(), "org-1", map[string]interface{}{"email": "ann@acme.com", "role": "member"})
	if err == nil {
		t.Fatal("an invitation answer was taken for a membership")
	}
}
