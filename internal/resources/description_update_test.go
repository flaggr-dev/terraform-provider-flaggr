package resources

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// These tests cover the description of every resource that has one besides
// flaggr_project (which has its own: project_update_test.go). Flaggr's PATCH
// routes keep a field an update doesn't name, and null doesn't clear it (the
// routes refuse it or ignore it), so the only way to remove a description is
// to send "". An update that leaves the field out changes nothing in Flaggr:
// the refresh after the apply reads the old text back and every plan shows the
// removal again.

// descriptionAPI is a Flaggr API for one entity that keeps its description
// the way the real PATCH routes do: a body that names description sets it (to
// "" too), a body that doesn't leaves it as it is, and null is refused. GET
// answers with whatever is stored, so a refresh after an update shows what the
// update did.
type descriptionAPI struct {
	// patchPath is where PATCH answers and getPath where GET does. Query, when
	// set, must be on both: a flag is found by its service and environment.
	patchPath, getPath string
	query              url.Values
	// entity is the entity as the API shows it with this stored description.
	entity func(description string) map[string]interface{}
	// patchReply and getReply wrap the entity the way each route answers
	// (nil: the entity itself).
	patchReply, getReply func(entity map[string]interface{}) interface{}

	mu      sync.Mutex
	stored  string
	patches []string
}

func (a *descriptionAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	reply := func(status int, v interface{}) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	wrap := func(shape func(map[string]interface{}) interface{}) interface{} {
		if shape == nil {
			return a.entity(a.stored)
		}
		return shape(a.entity(a.stored))
	}

	for key, want := range a.query {
		if r.URL.Query().Get(key) != want[0] {
			reply(http.StatusNotFound, map[string]string{"error": "Not found"})
			return
		}
	}
	switch {
	case r.Method == http.MethodPatch && r.URL.Path == a.patchPath:
		raw, _ := io.ReadAll(r.Body)
		a.patches = append(a.patches, string(raw))
		var body map[string]json.RawMessage
		if err := json.Unmarshal(raw, &body); err != nil {
			reply(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		if value, named := body["description"]; named {
			var text *string
			if err := json.Unmarshal(value, &text); err != nil || text == nil {
				reply(http.StatusBadRequest, map[string]string{"error": "Validation Error", "message": "description: expected string"})
				return
			}
			a.stored = *text
		}
		reply(http.StatusOK, wrap(a.patchReply))
	case r.Method == http.MethodGet && r.URL.Path == a.getPath:
		reply(http.StatusOK, wrap(a.getReply))
	default:
		reply(http.StatusNotFound, map[string]string{"error": "Not found"})
	}
}

func (a *descriptionAPI) client(t *testing.T) *client.Client {
	t.Helper()
	server := httptest.NewServer(a)
	t.Cleanup(server.Close)
	return client.NewClient(server.URL, "fgp_write")
}

// description is what Flaggr has stored.
func (a *descriptionAPI) description() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stored
}

// onlyPatch is the body of the one PATCH request the API got.
func (a *descriptionAPI) onlyPatch(t *testing.T) string {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.patches) != 1 {
		t.Fatalf("PATCH requests = %v, want exactly one", a.patches)
	}
	return a.patches[0]
}

// descriptionSent is the description a PATCH body names; sent is false when
// the body leaves it out.
func descriptionSent(t *testing.T, body string) (text string, sent bool) {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		t.Fatalf("PATCH body %s: %v", body, err)
	}
	value, sent := fields["description"]
	if !sent {
		return "", false
	}
	if err := json.Unmarshal(value, &text); err != nil {
		t.Fatalf("PATCH body %s: description is not a string: %v", body, err)
	}
	return text, true
}

// descriptionCase is one resource with a description, and the API that serves
// it.
type descriptionCase struct {
	name string
	// api serves the resource's entity, which starts with this description.
	api func(stored string) *descriptionAPI
	// resource is the resource, talking to the client.
	resource func(*client.Client) resource.Resource
	// model is the resource's model with this description. State and plan
	// differ in nothing else, as in an update of the description alone.
	model func(description types.String) interface{}
}

func descriptionCases() []descriptionCase {
	return []descriptionCase{
		{
			name: "flaggr_flag",
			api: func(stored string) *descriptionAPI {
				return &descriptionAPI{
					stored:    stored,
					patchPath: "/api/flags/dark-mode",
					getPath:   "/api/flags/dark-mode",
					query:     url.Values{"serviceId": {"svc-1"}, "environment": {"production"}},
					entity: func(description string) map[string]interface{} {
						return map[string]interface{}{
							"key": "dark-mode", "name": "Dark Mode", "description": description, "type": "boolean",
							"enabled": false, "defaultValue": false, "projectId": "p1", "serviceId": "svc-1",
							"environment": "production", "createdAt": "c", "updatedAt": "u2",
						}
					},
				}
			},
			resource: func(c *client.Client) resource.Resource { return &FlagResource{client: c} },
			model: func(description types.String) interface{} {
				return &FlagResourceModel{
					ID: types.StringValue("dark-mode:svc-1:production"), Key: types.StringValue("dark-mode"),
					Name: types.StringValue("Dark Mode"), Description: description, Type: types.StringValue("boolean"),
					Enabled: types.BoolValue(false), DefaultValue: types.StringValue("false"),
					ProjectID: types.StringValue("p1"), ServiceID: types.StringValue("svc-1"), Environment: types.StringValue("production"),
					IsPublic: types.BoolValue(false), CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
				}
			},
		},
		{
			name: "flaggr_service",
			api: func(stored string) *descriptionAPI {
				return &descriptionAPI{
					stored:    stored,
					patchPath: "/api/services/svc-1",
					getPath:   "/api/services/svc-1",
					entity: func(description string) map[string]interface{} {
						return map[string]interface{}{
							"id": "svc-1", "slug": "checkout", "projectId": "p1", "name": "Checkout",
							"description": description, "createdAt": "c", "updatedAt": "u2",
						}
					},
				}
			},
			resource: func(c *client.Client) resource.Resource { return &ServiceResource{client: c} },
			model: func(description types.String) interface{} {
				return &ServiceResourceModel{
					ID: types.StringValue("svc-1"), ProjectID: types.StringValue("p1"), Name: types.StringValue("Checkout"),
					Slug: types.StringValue("checkout"), Description: description,
					CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
				}
			},
		},
		{
			name: "flaggr_organization",
			api: func(stored string) *descriptionAPI {
				wrapped := func(entity map[string]interface{}) interface{} {
					return map[string]interface{}{"organization": entity}
				}
				return &descriptionAPI{
					stored:     stored,
					patchPath:  "/api/organizations/org-1",
					getPath:    "/api/organizations/org-1",
					patchReply: wrapped,
					getReply:   wrapped,
					entity: func(description string) map[string]interface{} {
						return map[string]interface{}{
							"id": "org-1", "name": "Acme", "slug": "acme", "description": description,
							"createdAt": "c", "updatedAt": "u2",
						}
					},
				}
			},
			resource: func(c *client.Client) resource.Resource { return &OrganizationResource{client: c} },
			model: func(description types.String) interface{} {
				return &OrganizationResourceModel{
					ID: types.StringValue("org-1"), Name: types.StringValue("Acme"), Slug: types.StringValue("acme"),
					Description: description, CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
				}
			},
		},
		{
			name: "flaggr_alert_rule",
			api: func(stored string) *descriptionAPI {
				return &descriptionAPI{
					stored:    stored,
					patchPath: "/api/projects/p1/alerts/r1",
					// The API only lists a project's rules: the provider reads one from the list.
					getPath: "/api/projects/p1/alerts",
					getReply: func(entity map[string]interface{}) interface{} {
						return map[string]interface{}{"rules": []interface{}{entity}}
					},
					entity: func(description string) map[string]interface{} {
						return map[string]interface{}{
							"id": "r1", "projectId": "p1", "name": "Errors", "description": description,
							"severity": "warning", "conditionType": "error_rate", "threshold": 5, "windowMinutes": 15,
							"channels": []interface{}{map[string]string{"channelId": "ch1", "channelName": "Ops"}},
							"enabled":  true, "cooldownMinutes": 15, "createdAt": "c", "updatedAt": "u2",
						}
					},
				}
			},
			resource: func(c *client.Client) resource.Resource { return &AlertRuleResource{client: c} },
			model: func(description types.String) interface{} {
				return &AlertRuleResourceModel{
					ID: types.StringValue("r1"), ProjectID: types.StringValue("p1"), Name: types.StringValue("Errors"),
					Description: description, Severity: types.StringValue("warning"), ConditionType: types.StringValue("error_rate"),
					Threshold: types.Float64Value(5), WindowMinutes: types.Int64Value(15),
					Channels: types.StringValue(`[{"channelId":"ch1","channelName":"Ops"}]`),
					Enabled:  types.BoolValue(true), CooldownMinutes: types.Int64Value(15),
					CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
				}
			},
		},
		{
			name: "flaggr_environment",
			api: func(stored string) *descriptionAPI {
				return &descriptionAPI{
					stored:    stored,
					patchPath: "/api/projects/p1/environments/qa",
					// The API only lists a project's environments: the provider reads one from the list.
					getPath: "/api/projects/p1/environments",
					getReply: func(entity map[string]interface{}) interface{} {
						return map[string]interface{}{"environments": []interface{}{entity}}
					},
					entity: func(description string) map[string]interface{} {
						return map[string]interface{}{
							"id": "env-1", "projectId": "p1", "slug": "qa", "name": "QA", "description": description,
							"color": "#10b981", "order": 30, "isDefault": false,
							"settings":  map[string]bool{"protectedEnvironment": false, "requireApproval": false},
							"createdAt": "c", "updatedAt": "u2",
						}
					},
				}
			},
			resource: func(c *client.Client) resource.Resource { return &EnvironmentResource{client: c} },
			model: func(description types.String) interface{} {
				return &EnvironmentResourceModel{
					ID: types.StringValue("env-1"), ProjectID: types.StringValue("p1"), Slug: types.StringValue("qa"),
					Name: types.StringValue("QA"), Description: description, Color: types.StringValue("#10b981"),
					Order: types.Int64Value(30), ProtectedEnvironment: types.BoolValue(false), RequireApproval: types.BoolValue(false),
					IsDefault: types.BoolValue(false), CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
				}
			},
		},
	}
}

// updateResource runs Update the way the framework does: the response starts
// with the prior state as its new state, so an Update that sets none (as a
// failed one doesn't) leaves the state as it was.
func updateResource(t *testing.T, r resource.Resource, planned, prior interface{}) resource.UpdateResponse {
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
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: state.Raw}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)
	return resp
}

// refreshResource runs Read on a state the way the framework does: the
// response starts with the state being read.
func refreshResource(t *testing.T, r resource.Resource, state tfsdk.State) tfsdk.State {
	t.Helper()
	resp := resource.ReadResponse{State: tfsdk.State{Schema: state.Schema, Raw: state.Raw}}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics)
	}
	return resp.State
}

func descriptionIn(t *testing.T, state tfsdk.State) types.String {
	t.Helper()
	var description types.String
	if diags := state.GetAttribute(context.Background(), path.Root("description"), &description); diags.HasError() {
		t.Fatalf("description: %v", diags)
	}
	return description
}

// A description removed from the configuration is cleared in Flaggr, so the
// refresh after the apply finds none and the next plan is empty. Before, the
// update left the description out, Flaggr kept the text, and the refresh read
// it back: every plan showed the removal again and no apply could finish it.
func TestUpdateClearsADescriptionRemovedFromTheConfiguration(t *testing.T) {
	for _, tc := range descriptionCases() {
		t.Run(tc.name, func(t *testing.T) {
			api := tc.api("Old text")
			r := tc.resource(api.client(t))

			resp := updateResource(t, r, tc.model(types.StringNull()), tc.model(types.StringValue("Old text")))
			if resp.Diagnostics.HasError() {
				t.Fatalf("update: %v", resp.Diagnostics)
			}

			body := api.onlyPatch(t)
			if text, sent := descriptionSent(t, body); !sent || text != "" {
				t.Errorf("PATCH body = %s: it must send an empty description, the only way to clear one", body)
			}
			if got := api.description(); got != "" {
				t.Errorf("Flaggr still has the description %q", got)
			}
			if got := descriptionIn(t, resp.State); !got.IsNull() {
				t.Errorf("state after the update: description = %s, want null, as configured", got)
			}
			if got := descriptionIn(t, refreshResource(t, r, resp.State)); !got.IsNull() {
				t.Errorf("state after a refresh: description = %s, want null: the plan would show the removal on every run", got)
			}
		})
	}
}

// A description in the configuration is sent as it is, including an empty one:
// it replaces the old text, and the refresh keeps the configured value.
func TestUpdateSendsTheConfiguredDescription(t *testing.T) {
	for _, tc := range descriptionCases() {
		for name, planned := range map[string]types.String{
			"a new description":    types.StringValue("New text"),
			"an empty description": types.StringValue(""),
		} {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				api := tc.api("Old text")
				r := tc.resource(api.client(t))

				resp := updateResource(t, r, tc.model(planned), tc.model(types.StringValue("Old text")))
				if resp.Diagnostics.HasError() {
					t.Fatalf("update: %v", resp.Diagnostics)
				}

				body := api.onlyPatch(t)
				if text, sent := descriptionSent(t, body); !sent || text != planned.ValueString() {
					t.Errorf("PATCH body = %s, want description %s", body, planned)
				}
				if got := descriptionIn(t, resp.State); !got.Equal(planned) {
					t.Errorf("state after the update: description = %s, want %s", got, planned)
				}
				if got := descriptionIn(t, refreshResource(t, r, resp.State)); !got.Equal(planned) {
					t.Errorf("state after a refresh: description = %s, want %s", got, planned)
				}
			})
		}
	}
}

// With no description to clear, an update leaves the field out as it always
// did: Flaggr has nothing to change, and a change request in an environment
// that needs approval doesn't list a description change.
func TestUpdateLeavesTheDescriptionOutWhenThereIsNothingToClear(t *testing.T) {
	for _, tc := range descriptionCases() {
		for name, prior := range map[string]types.String{
			"none":  types.StringNull(),
			"empty": types.StringValue(""),
		} {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				api := tc.api("")
				r := tc.resource(api.client(t))

				resp := updateResource(t, r, tc.model(types.StringNull()), tc.model(prior))
				if resp.Diagnostics.HasError() {
					t.Fatalf("update: %v", resp.Diagnostics)
				}

				body := api.onlyPatch(t)
				if _, sent := descriptionSent(t, body); sent {
					t.Errorf("PATCH body = %s: it names a description, but there was none to clear", body)
				}
				if got := descriptionIn(t, refreshResource(t, r, resp.State)); !got.IsNull() {
					t.Errorf("state after a refresh: description = %s, want null", got)
				}
			})
		}
	}
}

// setUpdateDescription decides from the plan and the prior state alone; the
// rows are what every resource's update does with a description.
func TestSetUpdateDescription(t *testing.T) {
	null, unknown, empty := types.StringNull(), types.StringUnknown(), types.StringValue("")
	oldText, newText := types.StringValue("Old text"), types.StringValue("New text")

	for _, tc := range []struct {
		name        string
		plan, state types.String
		sends       bool   // whether the body names a description
		want        string // the description it names
	}{
		{"a new description over the old one", newText, oldText, true, "New text"},
		{"an unchanged description", oldText, oldText, true, "Old text"},
		{"a description over none", newText, null, true, "New text"},
		{"an empty description over the old one", empty, oldText, true, ""},
		{"removed from the configuration", null, oldText, true, ""},
		{"removed after being empty", null, empty, false, ""},
		{"none before and now", null, null, false, ""},
		{"removed, prior description unknown", null, unknown, false, ""},
		{"not known until the apply", unknown, oldText, false, ""},
	} {
		input := map[string]interface{}{"name": "x"}
		setUpdateDescription(input, tc.plan, tc.state)

		got, sent := input["description"]
		if sent != tc.sends || (sent && got != tc.want) {
			t.Errorf("%s: input = %v, want description %q (sent: %t)", tc.name, input, tc.want, tc.sends)
		}
		if input["name"] != "x" {
			t.Errorf("%s: input = %v lost the name", tc.name, input)
		}
	}
}
