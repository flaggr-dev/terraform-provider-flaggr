package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// flagRoute behaves like GET, PATCH and DELETE /api/flags/{key}: the flag is
// found by the serviceId and environment query parameters, a missing
// serviceId is a 400, a missing environment means "development", and the
// body can't name either. It holds one flag: dark-mode in svc-1, production.
// With approval set, production answers PATCH with a change request (202).
type flagRoute struct {
	approval bool
	patches  []string
}

func (f *flagRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query()
	serviceID := query.Get("serviceId")
	environment := query.Get("environment")
	if environment == "" {
		environment = "development"
	}
	switch {
	case r.URL.Path != "/api/flags/dark-mode":
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
	case serviceID == "":
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Missing required query parameter: serviceId"}`))
	case serviceID != "svc-1" || environment != "production":
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Flag with key \"dark-mode\" not found"}`))
	case r.Method == http.MethodPatch && f.approval:
		body, _ := io.ReadAll(r.Body)
		f.patches = append(f.patches, string(body))
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message":"This environment requires approval. A change request has been created.","changeRequest":{"id":"cr-42","status":"pending"}}`))
	case r.Method == http.MethodPatch:
		body, _ := io.ReadAll(r.Body)
		f.patches = append(f.patches, string(body))
		_, _ = w.Write([]byte(`{"key":"dark-mode","name":"Dark Mode","type":"boolean","enabled":false,"defaultValue":false,"projectId":"p1","serviceId":"svc-1","environment":"production","createdAt":"c","updatedAt":"u2"}`))
	case r.Method == http.MethodGet:
		_, _ = w.Write([]byte(`{"key":"dark-mode","name":"Dark Mode","type":"boolean","enabled":true,"defaultValue":false,"projectId":"p1","serviceId":"svc-1","environment":"production","createdAt":"c","updatedAt":"u"}`))
	case r.Method == http.MethodDelete:
		_, _ = w.Write([]byte(`{"success":true}`))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func newFlagRoute(t *testing.T, route *flagRoute) *Client {
	t.Helper()
	server := httptest.NewServer(route)
	t.Cleanup(server.Close)
	return NewClient(server.URL, "fgp_test")
}

// v0.1.0 sent serviceId and environment only in the PATCH body, so every
// update failed with HTTP 400 (and would have targeted "development").
func TestUpdateFlagAddressesTheFlagInTheQuery(t *testing.T) {
	route := &flagRoute{}
	c := newFlagRoute(t, route)

	flag, err := c.UpdateFlag(context.Background(), "dark-mode", "svc-1", "production", map[string]interface{}{"enabled": false})
	if err != nil {
		t.Fatalf("UpdateFlag: %v", err)
	}
	if flag.Key != "dark-mode" || flag.Enabled || flag.Environment != "production" || flag.UpdatedAt != "u2" {
		t.Fatalf("UpdateFlag = %+v", flag)
	}
	if len(route.patches) != 1 {
		t.Fatalf("patches = %v", route.patches)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal([]byte(route.patches[0]), &sent); err != nil || sent["enabled"] != false {
		t.Fatalf("PATCH body = %s (%v)", route.patches[0], err)
	}
}

func TestGetAndDeleteFlagAddressTheFlagInTheQuery(t *testing.T) {
	c := newFlagRoute(t, &flagRoute{})
	ctx := context.Background()

	flag, err := c.GetFlag(ctx, "dark-mode", "svc-1", "production")
	if err != nil || flag.ServiceID != "svc-1" || !flag.Enabled {
		t.Fatalf("GetFlag = %+v, %v", flag, err)
	}
	if err := c.DeleteFlag(ctx, "dark-mode", "svc-1", "production"); err != nil {
		t.Fatalf("DeleteFlag: %v", err)
	}
	if _, err := c.GetFlag(ctx, "dark-mode", "svc-1", "staging"); !IsNotFoundError(err) {
		t.Fatalf("GetFlag in another environment: err = %v, want a 404", err)
	}
}

func TestUpdateFlagReportsAChangeRequest(t *testing.T) {
	route := &flagRoute{approval: true}
	c := newFlagRoute(t, route)

	flag, err := c.UpdateFlag(context.Background(), "dark-mode", "svc-1", "production", map[string]interface{}{"enabled": false})
	var changeRequest *ChangeRequestError
	if !errors.As(err, &changeRequest) || flag != nil {
		t.Fatalf("UpdateFlag = %+v, %v; want a *ChangeRequestError", flag, err)
	}
	if changeRequest.ChangeRequestID != "cr-42" || changeRequest.Environment != "production" {
		t.Fatalf("ChangeRequestError = %+v", changeRequest)
	}
	if got, want := err.Error(), "the production environment requires approval: Flaggr opened change request cr-42 instead of making the change"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
