package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The project routes answer {"project": {...}}. Decoding that into a bare
// Project left every field empty, so flaggr_project stored an empty ID.
func TestProjectCallsReadTheProjectWrapper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"My app","slug":"my-app","orgId":"org-1","createdAt":"c","updatedAt":"u"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/p1":
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"My app","slug":"my-app","orgId":"org-1","memberCount":3,"createdAt":"c","updatedAt":"u"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/by-slug/my-app":
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"My app","slug":"my-app","role":"owner","viaFloor":false,"createdAt":"c","updatedAt":"u"}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/projects/p1":
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"Renamed","slug":"my-app","createdAt":"c","updatedAt":"u2"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := NewClient(server.URL, "fgp_test")
	ctx := context.Background()

	created, err := c.CreateProject(ctx, map[string]interface{}{"name": "My app", "slug": "my-app"})
	if err != nil || created.ID != "p1" || created.OrganizationID != "org-1" || created.Slug != "my-app" {
		t.Fatalf("CreateProject = %+v, %v", created, err)
	}
	got, err := c.GetProject(ctx, "p1")
	if err != nil || got.ID != "p1" || got.Name != "My app" {
		t.Fatalf("GetProject = %+v, %v", got, err)
	}
	bySlug, err := c.GetProjectBySlug(ctx, "my-app")
	if err != nil || bySlug.ID != "p1" {
		t.Fatalf("GetProjectBySlug = %+v, %v", bySlug, err)
	}
	updated, err := c.UpdateProject(ctx, "p1", map[string]interface{}{"name": "Renamed"})
	if err != nil || updated.Name != "Renamed" || updated.UpdatedAt != "u2" {
		t.Fatalf("UpdateProject = %+v, %v", updated, err)
	}
}

func TestDecodeProjectAcceptsABareObjectAndRefusesOneWithoutAnID(t *testing.T) {
	bare, err := decodeProject([]byte(`{"id":"p2","name":"Bare","slug":"bare"}`))
	if err != nil || bare.ID != "p2" {
		t.Fatalf("bare = %+v, %v", bare, err)
	}
	for _, body := range []string{`{"project":{}}`, `{"error":"nope"}`, `{}`, `not json`} {
		if p, err := decodeProject([]byte(body)); err == nil {
			t.Fatalf("decodeProject(%s) = %+v, want an error", body, p)
		}
	}
}
