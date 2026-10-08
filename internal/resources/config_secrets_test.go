package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConfigFromAPIKeepsStateSecretsForRedactedFields(t *testing.T) {
	prior := types.StringValue(`{"channel":"#ops","url":"https://hooks.slack.com/services/T/B/x"}`)
	got := configFromAPI(json.RawMessage(`{"url":"[redacted]","channel":"#ops"}`), prior)
	if got != prior {
		t.Fatalf("config = %s, want the prior state unchanged", got)
	}
}

func TestConfigFromAPIIgnoresKeyOrderAndFormatting(t *testing.T) {
	prior := types.StringValue("{\n  \"b\": 1.0,\n  \"a\": [true, null]\n}")
	got := configFromAPI(json.RawMessage(`{"a":[true,null],"b":1}`), prior)
	if got != prior {
		t.Fatalf("config = %s, want the prior state unchanged", got)
	}
}

func TestConfigFromAPIReportsDriftAndStillKeepsSecrets(t *testing.T) {
	prior := types.StringValue(`{"channel":"#ops","url":"https://hooks.slack.com/services/T/B/x"}`)
	got := configFromAPI(json.RawMessage(`{"url":"[redacted]","channel":"#alerts"}`), prior)
	want := `{"channel":"#alerts","url":"https://hooks.slack.com/services/T/B/x"}`
	if got.ValueString() != want {
		t.Fatalf("config = %s, want %s", got, want)
	}
}

func TestConfigFromAPIWalksHeadersAndArrays(t *testing.T) {
	prior := types.StringValue(`{"headers":{"Authorization":"Bearer abc","X-Env":"prod"},"urls":["https://a","https://b"],"timeoutMs":5000}`)
	api := `{"headers":{"Authorization":"[redacted]","X-Env":"[redacted]"},"urls":["[redacted]","[redacted]"],"timeoutMs":5000}`
	if got := configFromAPI(json.RawMessage(api), prior); got != prior {
		t.Fatalf("config = %s, want the prior state unchanged", got)
	}
}

func TestConfigFromAPIDropsProviderKeysStateDoesNotSet(t *testing.T) {
	prior := types.StringValue(`{"apiKey":"dd-key","site":"datadoghq.com"}`)
	api := json.RawMessage(`{"apiKey":"[redacted]","site":"datadoghq.com","type":"datadog"}`)
	if got := configFromAPI(api, prior, "type"); got != prior {
		t.Fatalf("config = %s, want the prior state unchanged", got)
	}
	// A config that sets the key itself is compared with it.
	withType := types.StringValue(`{"apiKey":"dd-key","site":"datadoghq.com","type":"datadog"}`)
	if got := configFromAPI(api, withType, "type"); got != withType {
		t.Fatalf("config = %s, want %s", got, withType)
	}
}

func TestConfigFromAPIKeepsAPlaceholderWithNothingBehindIt(t *testing.T) {
	prior := types.StringValue(`{"channel":"#ops"}`)
	got := configFromAPI(json.RawMessage(`{"channel":"#ops","token":"[redacted]"}`), prior)
	if got.ValueString() != `{"channel":"#ops","token":"[redacted]"}` {
		t.Fatalf("config = %s", got)
	}
}

func TestConfigFromAPIStoresTheAPIValueWithoutUsablePriorState(t *testing.T) {
	api := json.RawMessage(`{"url":"[redacted]"}`)
	for name, prior := range map[string]types.String{
		"import":  types.StringNull(),
		"unknown": types.StringUnknown(),
		"invalid": types.StringValue(`{not json`),
	} {
		if got := configFromAPI(api, prior); got.ValueString() != string(api) {
			t.Fatalf("%s: config = %s, want the API's config", name, got)
		}
	}
}

func TestMisplacedPlaceholders(t *testing.T) {
	null := types.StringNull()
	cases := []struct {
		name   string
		config string
		prior  types.String
		want   []string
	}{
		{"create with a real secret", `{"url":"https://x"}`, null, nil},
		{"create with a placeholder", `{"url":"[redacted]"}`, null, []string{"url"}},
		{"nested and array placeholders", `{"headers":{"Authorization":"[redacted]"},"urls":["https://a","[redacted]"]}`, null, []string{"headers.Authorization", "urls[1]"}},
		{"placeholder carried over from state", `{"url":"[redacted]","channel":"#new"}`, types.StringValue(`{"url":"[redacted]","channel":"#ops"}`), nil},
		{"placeholder over a real state value", `{"url":"[redacted]"}`, types.StringValue(`{"url":"https://x"}`), []string{"url"}},
		{"placeholder at a new path", `{"url":"https://x","token":"[redacted]"}`, types.StringValue(`{"url":"https://x"}`), []string{"token"}},
		{"invalid JSON is reported elsewhere", `{`, null, nil},
	}
	for _, tc := range cases {
		got := misplacedPlaceholders(tc.config, tc.prior)
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if detail := placeholderErrorDetail([]string{"url"}); strings.Contains(detail, "https://") || !strings.Contains(detail, "at url") {
		t.Errorf("detail = %q", detail)
	}
}

// fakeFlaggr serves one alert channel and one metric source with redacted
// secrets, and records the bodies of PATCH requests.
func fakeFlaggr(t *testing.T, patches *[]string) *client.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/p1/alert-channels":
			_, _ = w.Write([]byte(`{"channels":[{"id":"ch1","projectId":"p1","name":"Ops","type":"slack","config":{"url":"[redacted]","channel":"#ops"},"enabled":true,"createdAt":"c","updatedAt":"u"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/p1/metric-sources":
			_, _ = w.Write([]byte(`{"sources":[{"id":"ms1","projectId":"p1","name":"Datadog","type":"datadog","config":{"apiKey":"[redacted]","appKey":"[redacted]","site":"datadoghq.com","type":"datadog"},"enabled":true,"createdAt":"c","updatedAt":"u"}]}`))
		case r.Method == http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			*patches = append(*patches, string(body))
			_, _ = w.Write([]byte(`{"id":"ch1","projectId":"p1","createdAt":"c","updatedAt":"u2"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return client.NewClient(server.URL, "fgp_member")
}

func schemaOf(t *testing.T, r resource.Resource) resource.SchemaResponse {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	return resp
}

func TestAlertChannelReadKeepsSecretsBelowTheAdminTier(t *testing.T) {
	ctx := context.Background()
	var patches []string
	r := &AlertChannelResource{client: fakeFlaggr(t, &patches)}
	s := schemaOf(t, r).Schema

	config := `{"channel":"#ops","url":"https://hooks.slack.com/services/T/B/x"}`
	state := tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &AlertChannelResourceModel{
		ID: types.StringValue("ch1"), ProjectID: types.StringValue("p1"), Name: types.StringValue("Ops"),
		Type: types.StringValue("slack"), Config: types.StringValue(config), Enabled: types.BoolValue(true),
		CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
	}); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}

	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics)
	}
	var got AlertChannelResourceModel
	resp.State.Get(ctx, &got)
	if got.Config.ValueString() != config {
		t.Fatalf("config after read = %s, want %s (no permanent diff)", got.Config.ValueString(), config)
	}
}

func TestMetricSourceReadKeepsSecretsAndIgnoresTheInjectedType(t *testing.T) {
	ctx := context.Background()
	var patches []string
	r := &MetricSourceResource{client: fakeFlaggr(t, &patches)}
	s := schemaOf(t, r).Schema

	config := `{"apiKey":"dd-api","appKey":"dd-app","site":"datadoghq.com"}`
	state := tfsdk.State{Schema: s}
	if diags := state.Set(ctx, &MetricSourceResourceModel{
		ID: types.StringValue("ms1"), ProjectID: types.StringValue("p1"), Name: types.StringValue("Datadog"),
		Type: types.StringValue("datadog"), Config: types.StringValue(config), Enabled: types.BoolValue(true),
		CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
	}); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}

	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read: %v", resp.Diagnostics)
	}
	var got MetricSourceResourceModel
	resp.State.Get(ctx, &got)
	if got.Config.ValueString() != config {
		t.Fatalf("config after read = %s, want %s", got.Config.ValueString(), config)
	}
}

func alertChannelUpdate(t *testing.T, r *AlertChannelResource, planned, prior string) resource.UpdateResponse {
	t.Helper()
	ctx := context.Background()
	s := schemaOf(t, r).Schema
	model := func(config string) *AlertChannelResourceModel {
		return &AlertChannelResourceModel{
			ID: types.StringValue("ch1"), ProjectID: types.StringValue("p1"), Name: types.StringValue("Ops"),
			Type: types.StringValue("slack"), Config: types.StringValue(config), Enabled: types.BoolValue(true),
			CreatedAt: types.StringValue("c"), UpdatedAt: types.StringValue("u"),
		}
	}
	plan := tfsdk.Plan{Schema: s}
	state := tfsdk.State{Schema: s}
	if diags := plan.Set(ctx, model(planned)); diags.HasError() {
		t.Fatalf("plan: %v", diags)
	}
	if diags := state.Set(ctx, model(prior)); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)
	return resp
}

func TestAlertChannelUpdateNeverSendsAPlaceholderAsAValue(t *testing.T) {
	var patches []string
	r := &AlertChannelResource{client: fakeFlaggr(t, &patches)}

	refused := alertChannelUpdate(t, r, `{"channel":"#ops","url":"[redacted]"}`, `{"channel":"#ops","url":"https://hooks.slack.com/services/T/B/x"}`)
	if !refused.Diagnostics.HasError() || len(patches) != 0 {
		t.Fatalf("diagnostics = %v, patches = %v: a placeholder over a real value must not be sent", refused.Diagnostics, patches)
	}
	if detail := refused.Diagnostics.Errors()[0].Detail(); strings.Contains(detail, "hooks.slack.com") {
		t.Fatalf("the error leaks the secret: %s", detail)
	}

	// Imported below the admin tier: the placeholder in state goes back as is,
	// and the API keeps the stored webhook URL.
	kept := alertChannelUpdate(t, r, `{"channel":"#alerts","url":"[redacted]"}`, `{"channel":"#ops","url":"[redacted]"}`)
	if kept.Diagnostics.HasError() || len(patches) != 1 {
		t.Fatalf("diagnostics = %v, patches = %v", kept.Diagnostics, patches)
	}
	if !strings.Contains(patches[0], `"url":"[redacted]"`) || !strings.Contains(patches[0], `"channel":"#alerts"`) {
		t.Fatalf("patch body = %s", patches[0])
	}
}

func TestAlertChannelCreateRefusesPlaceholders(t *testing.T) {
	ctx := context.Background()
	var patches []string
	r := &AlertChannelResource{client: fakeFlaggr(t, &patches)}
	s := schemaOf(t, r).Schema
	plan := tfsdk.Plan{Schema: s}
	if diags := plan.Set(ctx, &AlertChannelResourceModel{
		ID: types.StringUnknown(), ProjectID: types.StringValue("p1"), Name: types.StringValue("Ops"),
		Type: types.StringValue("webhook"), Config: types.StringValue(`{"url":"[redacted]"}`), Enabled: types.BoolValue(true),
		CreatedAt: types.StringUnknown(), UpdatedAt: types.StringUnknown(),
	}); diags.HasError() {
		t.Fatalf("plan: %v", diags)
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "at url") {
		t.Fatalf("diagnostics = %v, want a placeholder error for url", resp.Diagnostics)
	}
}
