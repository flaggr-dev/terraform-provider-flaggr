package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoRequestSendsBearerAndUserAgent(t *testing.T) {
	var gotAuth, gotUserAgent, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUserAgent = r.Header.Get("User-Agent")
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := NewClient(server.URL+"/", "fgp_test")
	c.UserAgent = "terraform-provider-flaggr/test"

	body, status, err := c.doRequest(context.Background(), http.MethodGet, "/api/users/me", nil)
	if err != nil {
		t.Fatalf("doRequest returned error: %v", err)
	}
	if status != http.StatusOK || string(body) != `{"ok":true}` {
		t.Fatalf("status = %d, body = %q", status, body)
	}
	if gotAuth != "Bearer fgp_test" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer fgp_test")
	}
	if gotUserAgent != "terraform-provider-flaggr/test" {
		t.Fatalf("User-Agent = %q, want %q", gotUserAgent, "terraform-provider-flaggr/test")
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
}

func TestDoRequestLeavesDefaultUserAgentWhenUnset(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	if _, _, err := NewClient(server.URL, "fgr_test").doRequest(context.Background(), http.MethodGet, "/x", nil); err != nil {
		t.Fatalf("doRequest returned error: %v", err)
	}
	if gotUserAgent == "" {
		t.Fatal("expected Go's default User-Agent when none is configured")
	}
}

func TestDoRequestSurfacesForbiddenError(t *testing.T) {
	const message = "Forbidden: personal access token lacks the admin tier — create an admin token under Profile → Personal access tokens"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
	}))
	defer server.Close()

	_, status, err := NewClient(server.URL, "fgp_test").doRequest(context.Background(), http.MethodPost, "/api/organizations", map[string]string{"name": "Acme"})
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.Message != message {
		t.Fatalf("APIError = %+v", apiErr)
	}
	if IsNotFoundError(err) {
		t.Fatal("a 403 must not read as not found")
	}
}
