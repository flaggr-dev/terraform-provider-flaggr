package resources

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// fakeFlagAPI answers PATCH /api/flags/dark-mode the way Flaggr does: it finds
// the flag by the serviceId and environment query parameters (400 without
// serviceId, "development" without environment) and ignores them in the body.
// With approval set, it opens a change request (202) instead of updating.
type fakeFlagAPI struct {
	approval bool
	queries  []map[string][]string
	bodies   []string
}

func (f *fakeFlagAPI) client(t *testing.T) *client.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPatch || r.URL.Path != "/api/flags/dark-mode" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.queries = append(f.queries, r.URL.Query())
		f.bodies = append(f.bodies, string(body))

		query := r.URL.Query()
		environment := query.Get("environment")
		if environment == "" {
			environment = "development"
		}
		switch {
		case query.Get("serviceId") == "":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Missing required query parameter: serviceId"}`))
		case query.Get("serviceId") != "svc-1" || environment != "production":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Flag with key \"dark-mode\" not found"}`))
		case f.approval:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"message":"This environment requires approval. A change request has been created.","changeRequest":{"id":"cr-42","status":"pending"}}`))
		default:
			_, _ = w.Write([]byte(`{"key":"dark-mode","name":"Dark Mode","type":"boolean","enabled":false,"defaultValue":false,"projectId":"p1","serviceId":"svc-1","environment":"production","createdAt":"c","updatedAt":"u2"}`))
		}
	}))
	t.Cleanup(server.Close)
	return client.NewClient(server.URL, "fgp_write")
}

func darkModeFlag(enabled bool) *FlagResourceModel {
	return &FlagResourceModel{
		ID: types.StringValue("dark-mode:svc-1:production"), Key: types.StringValue("dark-mode"),
		Name: types.StringValue("Dark Mode"), Description: types.StringNull(), Type: types.StringValue("boolean"),
		Enabled: types.BoolValue(enabled), DefaultValue: types.StringValue("false"),
		ProjectID: types.StringValue("p1"), ServiceID: types.StringValue("svc-1"), Environment: types.StringValue("production"),
		IsPublic: types.BoolValue(false), CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
	}
}

// flagUpdate runs Update the way the framework does: the response starts with
// a null state, and an error with no new state keeps the prior one.
func flagUpdate(t *testing.T, r *FlagResource, planned, prior *FlagResourceModel) resource.UpdateResponse {
	t.Helper()
	ctx := context.Background()
	s := schemaOf(t, r).Schema
	plan := tfsdk.Plan{Schema: s}
	state := tfsdk.State{Schema: s}
	if diags := plan.Set(ctx, planned); diags.HasError() {
		t.Fatalf("plan: %v", diags)
	}
	if diags := state.Set(ctx, prior); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)
	return resp
}

func TestFlagUpdateNamesTheFlagsServiceAndEnvironmentInTheQuery(t *testing.T) {
	api := &fakeFlagAPI{}
	r := &FlagResource{client: api.client(t)}

	resp := flagUpdate(t, r, darkModeFlag(false), darkModeFlag(true))
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	if len(api.queries) != 1 {
		t.Fatalf("PATCH requests = %d, want 1", len(api.queries))
	}
	if got := api.queries[0]; len(got["serviceId"]) != 1 || got["serviceId"][0] != "svc-1" || len(got["environment"]) != 1 || got["environment"][0] != "production" {
		t.Fatalf("PATCH query = %v, want serviceId=svc-1 and environment=production", got)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(api.bodies[0]), &body); err != nil {
		t.Fatalf("PATCH body %s: %v", api.bodies[0], err)
	}
	if _, ok := body["serviceId"]; ok {
		t.Errorf("PATCH body names serviceId, which the API ignores: %s", api.bodies[0])
	}
	if _, ok := body["environment"]; ok {
		t.Errorf("PATCH body names environment, which the API ignores: %s", api.bodies[0])
	}
	if body["enabled"] != false || body["name"] != "Dark Mode" {
		t.Errorf("PATCH body = %s", api.bodies[0])
	}

	var got FlagResourceModel
	if diags := resp.State.Get(context.Background(), &got); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}
	if got.Enabled.ValueBool() || got.UpdatedAt.ValueString() != "u2" || got.ID.ValueString() != "dark-mode:svc-1:production" {
		t.Fatalf("state after update = %+v", got)
	}
}

func TestFlagUpdateNeedingApprovalFailsAndKeepsThePriorState(t *testing.T) {
	api := &fakeFlagAPI{approval: true}
	r := &FlagResource{client: api.client(t)}

	resp := flagUpdate(t, r, darkModeFlag(false), darkModeFlag(true))
	if !resp.Diagnostics.HasError() {
		t.Fatal("an update that opened a change request must fail: the flag didn't change")
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	for _, want := range []string{"production environment requires approval", "change request cr-42", "opens another change request"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q doesn't mention %q", detail, want)
		}
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("Update set a new state; with none, Terraform keeps the prior state, which still matches the flag")
	}
}
