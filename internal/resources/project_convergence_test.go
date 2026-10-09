package resources_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	tfprovider "github.com/flaggr-dev/terraform-provider-flaggr/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests drive flaggr_project through the provider protocol the way
// Terraform does — plan, apply, refresh, plan again — against a fake Flaggr
// API that stores what it is sent. A change converges when the refresh after
// the apply reads back the state the apply wrote and the next plan is empty.

// apiProject is a project as the fake API stores it. Description is nil for
// a project created without one; a PATCH stores the string it is sent.
type apiProject struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
	OrgID       string  `json:"orgId,omitempty"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// fakeProjectsAPI serves /api/projects like Flaggr: POST creates the project
// in the requested organization, or defaultOrg ("" for an account in none),
// with a new ID; PATCH validates its body like updateProjectSchema (orgId and
// other unknown keys are dropped, an empty body and a null are refused) and
// changes only the fields it is sent; DELETE deletes the project.
type fakeProjectsAPI struct {
	mu         sync.Mutex
	projects   map[string]*apiProject
	defaultOrg string
	writes     []string // "POST {...}", "PATCH {...}" and "DELETE <id>"
	reads      int      // GET requests
	created    int
	updates    int
}

func newFakeProjectsAPI(projects ...apiProject) *fakeProjectsAPI {
	api := &fakeProjectsAPI{projects: map[string]*apiProject{}}
	for i := range projects {
		api.projects[projects[i].ID] = &projects[i]
	}
	return api
}

func (f *fakeProjectsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(r.Body)
	reply := func(status int, v interface{}) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/projects" {
		f.writes = append(f.writes, "POST "+string(body))
		var in struct {
			Name        string  `json:"name"`
			Slug        string  `json:"slug"`
			Description *string `json:"description"`
			OrgID       string  `json:"orgId"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			reply(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		f.created++
		p := &apiProject{
			ID: fmt.Sprintf("p%d", 100+f.created), Name: in.Name, Slug: in.Slug,
			Description: in.Description, OrgID: in.OrgID, CreatedAt: "c", UpdatedAt: "u0",
		}
		if p.OrgID == "" {
			p.OrgID = f.defaultOrg
		}
		f.projects[p.ID] = p
		reply(http.StatusCreated, map[string]interface{}{"project": p})
		return
	}

	p, ok := f.projects[strings.TrimPrefix(r.URL.Path, "/api/projects/")]
	if !ok {
		reply(http.StatusNotFound, map[string]string{"error": "Project not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		f.reads++
		reply(http.StatusOK, map[string]interface{}{"project": p})
	case http.MethodDelete:
		f.writes = append(f.writes, "DELETE "+p.ID)
		delete(f.projects, p.ID)
		reply(http.StatusOK, map[string]bool{"success": true})
	case http.MethodPatch:
		f.writes = append(f.writes, "PATCH "+string(body))
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			reply(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		fields := map[string]string{}
		for _, key := range []string{"name", "slug", "description"} {
			value, sent := raw[key]
			if !sent {
				continue
			}
			var s *string
			if err := json.Unmarshal(value, &s); err != nil || s == nil {
				reply(http.StatusBadRequest, map[string]string{"error": "Validation Error", "message": key + ": expected string"})
				return
			}
			fields[key] = *s
		}
		if len(fields) == 0 {
			reply(http.StatusBadRequest, map[string]string{"error": "Validation Error", "message": "At least one field must be provided for update"})
			return
		}
		for key, value := range fields {
			switch key {
			case "name":
				p.Name = value
			case "slug":
				p.Slug = value
			case "description":
				description := value
				p.Description = &description
			}
		}
		f.updates++
		p.UpdatedAt = fmt.Sprintf("u%d", f.updates)
		reply(http.StatusOK, map[string]interface{}{"project": p})
	default:
		reply(http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (f *fakeProjectsAPI) project(t *testing.T, id string) apiProject {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.projects[id]
	if !ok {
		t.Fatalf("project %s isn't in the fake API", id)
	}
	return *p
}

func (f *fakeProjectsAPI) writesSoFar() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...)
}

func (f *fakeProjectsAPI) readsSoFar() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

// resourceHarness talks to the real provider over the protocol, configured
// against a fake API, about one of its resource types.
type resourceHarness struct {
	t        *testing.T
	typeName string
	server   tfprotov6.ProviderServer
	schema   *tfprotov6.Schema
}

func newProjectHarness(t *testing.T, api *fakeProjectsAPI) *resourceHarness {
	t.Helper()
	return newResourceHarness(t, "flaggr_project", api)
}

// newResourceHarness is a harness for one resource type of the provider,
// configured against the fake API that the handler serves.
func newResourceHarness(t *testing.T, typeName string, api http.Handler) *resourceHarness {
	t.Helper()
	ctx := context.Background()
	httpServer := httptest.NewServer(api)
	t.Cleanup(httpServer.Close)

	server, err := providerserver.NewProtocol6WithError(tfprovider.New("test")())()
	if err != nil {
		t.Fatalf("provider server: %v", err)
	}
	schemas, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil || hasError(schemas.Diagnostics) {
		t.Fatalf("provider schema: %v %s", err, diagnostics(schemas.Diagnostics))
	}
	providerConfig := map[string]tftypes.Value{}
	for _, a := range schemas.Provider.Block.Attributes {
		providerConfig[a.Name] = tftypes.NewValue(a.Type, nil)
	}
	providerConfig["api_url"] = tftypes.NewValue(tftypes.String, httpServer.URL)
	providerConfig["api_token"] = tftypes.NewValue(tftypes.String, "fgp_test")
	providerType := schemas.Provider.ValueType()
	config, err := tfprotov6.NewDynamicValue(providerType, tftypes.NewValue(providerType, providerConfig))
	if err != nil {
		t.Fatalf("provider config: %v", err)
	}
	configured, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{TerraformVersion: "1.9.0", Config: &config})
	if err != nil || hasError(configured.Diagnostics) {
		t.Fatalf("configure provider: %v %s", err, diagnostics(configured.Diagnostics))
	}
	schema, ok := schemas.ResourceSchemas[typeName]
	if !ok {
		t.Fatalf("the provider has no %s resource", typeName)
	}
	return &resourceHarness{t: t, typeName: typeName, server: server, schema: schema}
}

func (h *resourceHarness) dynamic(v tftypes.Value) *tfprotov6.DynamicValue {
	h.t.Helper()
	dv, err := tfprotov6.NewDynamicValue(h.schema.ValueType(), v)
	if err != nil {
		h.t.Fatalf("encoding %s: %v", v, err)
	}
	return &dv
}

func (h *resourceHarness) value(dv *tfprotov6.DynamicValue) tftypes.Value {
	h.t.Helper()
	if dv == nil {
		return tftypes.NewValue(h.schema.ValueType(), nil)
	}
	v, err := dv.Unmarshal(h.schema.ValueType())
	if err != nil {
		h.t.Fatalf("decoding a flaggr_project value: %v", err)
	}
	return v
}

// noResource is the prior state of a resource that doesn't exist yet.
func (h *resourceHarness) noResource() tftypes.Value {
	return tftypes.NewValue(h.schema.ValueType(), nil)
}

// config is a block of the resource setting the given attributes. A value is
// given as a string, whatever the attribute's type: "true" for a bool, "30"
// for a number.
func (h *resourceHarness) config(attrs map[string]string) tftypes.Value {
	h.t.Helper()
	values := map[string]tftypes.Value{}
	for _, a := range h.schema.Block.Attributes {
		values[a.Name] = tftypes.NewValue(a.Type, nil)
		v, ok := attrs[a.Name]
		if !ok {
			continue
		}
		switch {
		case a.Type.Equal(tftypes.Bool):
			b, err := strconv.ParseBool(v)
			if err != nil {
				h.t.Fatalf("%s is a bool, not %q", a.Name, v)
			}
			values[a.Name] = tftypes.NewValue(tftypes.Bool, b)
		case a.Type.Equal(tftypes.Number):
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				h.t.Fatalf("%s is a number, not %q", a.Name, v)
			}
			values[a.Name] = tftypes.NewValue(tftypes.Number, n)
		default:
			values[a.Name] = tftypes.NewValue(tftypes.String, v)
		}
	}
	return tftypes.NewValue(h.schema.ValueType(), values)
}

// proposedNew is Terraform's proposed new state for this flat schema: the
// configuration, except that a Computed attribute the configuration leaves
// null keeps its prior value.
func (h *resourceHarness) proposedNew(prior, config tftypes.Value) tftypes.Value {
	cfg, pri := attrs(h.t, config), attrs(h.t, prior)
	values := map[string]tftypes.Value{}
	for _, a := range h.schema.Block.Attributes {
		values[a.Name] = cfg[a.Name]
		if a.Computed && cfg[a.Name].IsNull() {
			values[a.Name] = tftypes.NewValue(a.Type, nil)
			if pri != nil {
				values[a.Name] = pri[a.Name]
			}
		}
	}
	return tftypes.NewValue(h.schema.ValueType(), values)
}

type resourcePlan struct {
	planned tftypes.Value
	private []byte
	diags   []*tfprotov6.Diagnostic
}

func (h *resourceHarness) plan(prior, config tftypes.Value) resourcePlan {
	h.t.Helper()
	resp, err := h.server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         h.typeName,
		PriorState:       h.dynamic(prior),
		ProposedNewState: h.dynamic(h.proposedNew(prior, config)),
		Config:           h.dynamic(config),
	})
	if err != nil {
		h.t.Fatalf("plan: %v", err)
	}
	if hasError(resp.Diagnostics) {
		return resourcePlan{diags: resp.Diagnostics}
	}
	planned := h.value(resp.PlannedState)
	// Terraform's own check of a plan: an attribute plans its configured
	// value, or its prior one when it is set in both, unless it is Computed
	// and either Computed-only or not configured.
	cfg, pri, pln := attrs(h.t, config), attrs(h.t, prior), attrs(h.t, planned)
	for _, a := range h.schema.Block.Attributes {
		c, p := cfg[a.Name], pln[a.Name]
		switch {
		case p.Equal(c):
		case pri != nil && p.Equal(pri[a.Name]) && !pri[a.Name].IsNull() && !c.IsNull():
		case a.Computed && (!a.Optional || c.IsNull()):
		default:
			h.t.Fatalf("invalid plan: %s planned %s for configured %s", a.Name, p, c)
		}
	}
	return resourcePlan{planned: planned, private: resp.PlannedPrivate, diags: resp.Diagnostics}
}

func (h *resourceHarness) apply(prior, config tftypes.Value, plan resourcePlan) (tftypes.Value, []*tfprotov6.Diagnostic) {
	h.t.Helper()
	resp, err := h.server.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{
		TypeName:       h.typeName,
		PriorState:     h.dynamic(prior),
		PlannedState:   h.dynamic(plan.planned),
		Config:         h.dynamic(config),
		PlannedPrivate: plan.private,
	})
	if err != nil {
		h.t.Fatalf("apply: %v", err)
	}
	state := h.value(resp.NewState)
	if hasError(resp.Diagnostics) {
		return state, resp.Diagnostics
	}
	// Terraform refuses a new state with unknown values, or one that differs
	// from a value the plan knew.
	if !state.IsFullyKnown() {
		h.t.Fatalf("apply left unknown values in the state: %s", state)
	}
	planned, got := attrs(h.t, plan.planned), attrs(h.t, state)
	for name, p := range planned {
		if p.IsKnown() && !p.Equal(got[name]) {
			h.t.Fatalf("inconsistent result after apply: %s was planned as %s, but now %s", name, p, got[name])
		}
	}
	return state, resp.Diagnostics
}

// destroy is the delete half of a replacement: Terraform applies a null
// planned state over the prior one.
func (h *resourceHarness) destroy(prior tftypes.Value) {
	h.t.Helper()
	resp, err := h.server.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{
		TypeName:     h.typeName,
		PriorState:   h.dynamic(prior),
		PlannedState: h.dynamic(h.noResource()),
		Config:       h.dynamic(h.noResource()),
	})
	if err != nil || hasError(resp.Diagnostics) {
		h.t.Fatalf("destroy: %v %s", err, diagnostics(resp.Diagnostics))
	}
	if state := h.value(resp.NewState); !state.IsNull() {
		h.t.Fatalf("destroy left %s", state)
	}
}

func (h *resourceHarness) read(state tftypes.Value) tftypes.Value {
	h.t.Helper()
	resp, err := h.server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{
		TypeName:     h.typeName,
		CurrentState: h.dynamic(state),
	})
	if err != nil || hasError(resp.Diagnostics) {
		h.t.Fatalf("refresh: %v %s", err, diagnostics(resp.Diagnostics))
	}
	return h.value(resp.NewState)
}

// importResource is `terraform import`: the imported state, then a refresh.
func (h *resourceHarness) importResource(id string) tftypes.Value {
	h.t.Helper()
	resp, err := h.server.ImportResourceState(context.Background(), &tfprotov6.ImportResourceStateRequest{
		TypeName: h.typeName,
		ID:       id,
	})
	if err != nil || hasError(resp.Diagnostics) || len(resp.ImportedResources) != 1 {
		h.t.Fatalf("import: %v %s", err, diagnostics(resp.Diagnostics))
	}
	return h.read(h.value(resp.ImportedResources[0].State))
}

// applyAndConverge is `terraform apply` of a configuration with changes,
// followed by `terraform plan`: the refresh must read back what the apply
// wrote, and the plan must then be empty. It returns the refreshed state.
func (h *resourceHarness) applyAndConverge(prior, config tftypes.Value) tftypes.Value {
	h.t.Helper()
	plan := h.plan(prior, config)
	if hasError(plan.diags) {
		h.t.Fatalf("plan: %s", diagnostics(plan.diags))
	}
	if plan.planned.Equal(prior) {
		h.t.Fatalf("the plan changes nothing in %s", prior)
	}
	state, diags := h.apply(prior, config, plan)
	if hasError(diags) {
		h.t.Fatalf("apply: %s", diagnostics(diags))
	}
	refreshed := h.read(state)
	if !refreshed.Equal(state) {
		h.t.Fatalf("the refresh after apply reads\n%s\nbut the apply wrote\n%s", refreshed, state)
	}
	h.planIsEmpty(refreshed, config)
	return refreshed
}

func (h *resourceHarness) planIsEmpty(state, config tftypes.Value) {
	h.t.Helper()
	plan := h.plan(state, config)
	if hasError(plan.diags) {
		h.t.Fatalf("plan: %s", diagnostics(plan.diags))
	}
	if !plan.planned.Equal(state) {
		h.t.Fatalf("the plan isn't empty: it plans\n%s\nover\n%s", plan.planned, state)
	}
}

func attrs(t *testing.T, v tftypes.Value) map[string]tftypes.Value {
	t.Helper()
	if v.IsNull() {
		return nil
	}
	var m map[string]tftypes.Value
	if err := v.As(&m); err != nil {
		t.Fatalf("reading %s: %v", v, err)
	}
	return m
}

func withAttr(t *testing.T, v tftypes.Value, name string, value tftypes.Value) tftypes.Value {
	t.Helper()
	m := attrs(t, v)
	m[name] = value
	return tftypes.NewValue(v.Type(), m)
}

func hasError(diags []*tfprotov6.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func diagnostics(diags []*tfprotov6.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		fmt.Fprintf(&b, "[%s: %s %s]", d.Attribute, d.Summary, d.Detail)
	}
	return b.String()
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

var nullString = tftypes.NewValue(tftypes.String, nil)

func strPtr(s string) *string { return &s }

// myApp is a project with a description in organization org-1.
func myApp() apiProject {
	return apiProject{ID: "p1", Name: "My app", Slug: "my-app", Description: strPtr("Flags"), OrgID: "org-1", CreatedAt: "c", UpdatedAt: "u0"}
}

// CR-5: removing the description from the configuration clears it in
// Flaggr, so the apply converges instead of recording a removal the API never
// made (the refresh used to put the description back on every plan). The
// imported project's description was set outside Terraform: a configuration
// without a description clears that one too, as the README warns.
func TestProjectRemovingTheDescriptionConverges(t *testing.T) {
	api := newFakeProjectsAPI(myApp())
	h := newProjectHarness(t, api)

	state := h.applyAndConverge(h.importResource("p1"), h.config(map[string]string{"name": "My app", "slug": "my-app"}))

	if got := attrs(t, state)["description"]; !got.IsNull() {
		t.Fatalf("description = %s, want none", got)
	}
	if got := api.project(t, "p1").Description; got == nil || *got != "" {
		t.Fatalf("the API's description = %v, want it cleared", got)
	}
	if writes := api.writesSoFar(); len(writes) != 1 || writes[0] != `PATCH {"description":""}` {
		t.Fatalf("writes = %v", writes)
	}
}

// CR-5: the plan refuses to move a project to another organization, instead
// of an apply that reports a change it never made and a plan that never
// converges.
func TestProjectOrganizationChangeIsRefusedAtPlan(t *testing.T) {
	api := newFakeProjectsAPI(myApp())
	h := newProjectHarness(t, api)
	state := h.importResource("p1")
	reads := api.readsSoFar()

	plan := h.plan(state, h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags", "organization_id": "org-2"}))
	if !hasError(plan.diags) {
		t.Fatalf("the plan moves the project to org-2: %s", plan.planned)
	}
	d := plan.diags[0]
	if d.Summary != "A project can't move to another organization" ||
		!d.Attribute.Equal(tftypes.NewAttributePath().WithAttributeName("organization_id")) ||
		!strings.Contains(d.Detail, `belongs to organization "org-1"`) ||
		!strings.Contains(d.Detail, `create a new flaggr_project in organization "org-2"`) {
		t.Fatalf("diagnostic = %s", diagnostics(plan.diags))
	}
	if writes := api.writesSoFar(); len(writes) != 0 {
		t.Fatalf("writes = %v", writes)
	}

	// Its own organization, or none configured, plans nothing.
	h.planIsEmpty(state, h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags", "organization_id": "org-1"}))
	h.planIsEmpty(state, h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags"}))

	// With an organization recorded the plan compares it: no request.
	if got := api.readsSoFar(); got != reads {
		t.Fatalf("the plans read the project %d times", got-reads)
	}
}

// A state with no organization recorded (written without one, and planned
// with -refresh=false: a refresh records the project's) takes the project's
// organization from the configuration without a PATCH, and converges.
func TestProjectOrganizationOverNoneRecordedConverges(t *testing.T) {
	api := newFakeProjectsAPI(myApp())
	h := newProjectHarness(t, api)
	prior := withAttr(t, h.importResource("p1"), "organization_id", nullString)

	state := h.applyAndConverge(prior, h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags", "organization_id": "org-1"}))

	if got := attrs(t, state)["organization_id"]; !got.Equal(str("org-1")) {
		t.Fatalf("organization_id = %s", got)
	}
	if writes := api.writesSoFar(); len(writes) != 0 {
		t.Fatalf("writes = %v", writes)
	}
}

// CR-5: with no organization recorded the plan modifier has nothing to
// compare, so the plan asks Flaggr for the project's own and refuses another
// one, rather than recording it or planning an update the apply refuses.
// (The harness plans without a refresh, like -refresh=false: a refresh would
// record org-1, and the plan modifier would refuse org-2.)
func TestProjectOrganizationOverNoneRecordedMustBeTheProjects(t *testing.T) {
	api := newFakeProjectsAPI(myApp())
	h := newProjectHarness(t, api)
	prior := withAttr(t, h.importResource("p1"), "organization_id", nullString)
	config := h.config(map[string]string{"name": "Renamed", "slug": "my-app", "description": "Flags", "organization_id": "org-2"})

	plan := h.plan(prior, config)
	if !hasError(plan.diags) || plan.diags[0].Summary != "A project can't move to another organization" ||
		!plan.diags[0].Attribute.Equal(tftypes.NewAttributePath().WithAttributeName("organization_id")) ||
		!strings.Contains(plan.diags[0].Detail, `belongs to organization "org-1"`) {
		t.Fatalf("plan = %s, diagnostics %s", plan.planned, diagnostics(plan.diags))
	}
	if writes := api.writesSoFar(); len(writes) != 0 {
		t.Fatalf("writes = %v (the rename went through without the organization)", writes)
	}
}

// Review of CR-5: a project in no organization records none, so every plan
// of a configured organization showed an update and every apply refused it
// (Flaggr can't add an existing project to an organization). The plan asks
// Flaggr and refuses it instead, and asks nothing when no organization is
// configured.
func TestProjectInNoOrganizationRefusesOneAtPlan(t *testing.T) {
	inNone := myApp()
	inNone.OrgID = ""
	api := newFakeProjectsAPI(inNone)
	h := newProjectHarness(t, api)
	state := h.importResource("p1")
	if got := attrs(t, state)["organization_id"]; !got.IsNull() {
		t.Fatalf("organization_id = %s, want none", got)
	}

	plan := h.plan(state, h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags", "organization_id": "org-1"}))
	if !hasError(plan.diags) || plan.diags[0].Summary != "A project can't move to another organization" ||
		!plan.diags[0].Attribute.Equal(tftypes.NewAttributePath().WithAttributeName("organization_id")) ||
		!strings.Contains(plan.diags[0].Detail, "This project doesn't belong to an organization") ||
		!strings.Contains(plan.diags[0].Detail, "to destroy the project, remove organization_id first") {
		t.Fatalf("plan = %s, diagnostics %s", plan.planned, diagnostics(plan.diags))
	}
	if writes := api.writesSoFar(); len(writes) != 0 {
		t.Fatalf("writes = %v", writes)
	}

	reads := api.readsSoFar()
	h.planIsEmpty(state, h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags"}))
	if got := api.readsSoFar(); got != reads {
		t.Fatalf("a plan without an organization read the project %d times", got-reads)
	}
}

// Review of CR-5: Terraform plans the configuration against the project
// before it plans a replacement for `terraform apply -replace`, and in the
// refresh before `terraform destroy`, so the refusal stops both while
// organization_id names another organization; the diagnostic says so, and
// points to `terraform taint`. A tainted project is planned as a new one:
// destroyed (which frees its slug), created in the other organization, and
// the result converges.
func TestProjectTaintedIsRecreatedInAnotherOrganization(t *testing.T) {
	api := newFakeProjectsAPI(myApp())
	h := newProjectHarness(t, api)
	state := h.importResource("p1")
	config := h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags", "organization_id": "org-2"})

	// The plan `terraform apply -replace` and `terraform destroy` start with.
	plan := h.plan(state, config)
	if !hasError(plan.diags) ||
		!strings.Contains(plan.diags[0].Detail, "run `terraform taint` on this resource, then apply") ||
		!strings.Contains(plan.diags[0].Detail, "this check also stops `terraform apply -replace` and `terraform destroy`") ||
		!strings.Contains(plan.diags[0].Detail, `to destroy the project, set organization_id back to "org-1" (or remove it) first`) {
		t.Fatalf("diagnostics = %s", diagnostics(plan.diags))
	}

	// After `terraform taint`: destroy, then create (applyAndConverge plans
	// the new project over none, as Terraform does for a tainted one).
	h.destroy(state)
	replaced := h.applyAndConverge(h.noResource(), config)

	if got := attrs(t, replaced); !got["organization_id"].Equal(str("org-2")) || got["id"].Equal(str("p1")) {
		t.Fatalf("replacement = %s", replaced)
	}
	writes := api.writesSoFar()
	if len(writes) != 2 || writes[0] != "DELETE p1" || !strings.HasPrefix(writes[1], "POST ") || !strings.Contains(writes[1], `"orgId":"org-2"`) {
		t.Fatalf("writes = %v", writes)
	}
}

// A project created by an account in no organization has none: the state
// records null, not the plan's "known after apply" (which Terraform rejects
// after an apply).
func TestProjectCreatedConvergesWithOrWithoutAnOrganization(t *testing.T) {
	for _, org := range []string{"", "org-1"} {
		api := newFakeProjectsAPI()
		api.defaultOrg = org
		h := newProjectHarness(t, api)

		state := h.applyAndConverge(h.noResource(), h.config(map[string]string{"name": "My app", "slug": "my-app"}))

		want := nullString
		if org != "" {
			want = str(org)
		}
		if got := attrs(t, state)["organization_id"]; !got.Equal(want) {
			t.Fatalf("default organization %q: organization_id = %s", org, got)
		}
	}
}

// A description cleared outside Terraform shows up in the next plan (the
// refresh used to keep the old one), and applying it puts it back.
func TestProjectDescriptionClearedOutsideTerraformIsPutBack(t *testing.T) {
	api := newFakeProjectsAPI(myApp())
	h := newProjectHarness(t, api)
	state := h.importResource("p1")
	config := h.config(map[string]string{"name": "My app", "slug": "my-app", "description": "Flags"})

	api.mu.Lock()
	api.projects["p1"].Description = strPtr("") // cleared in the dashboard
	api.mu.Unlock()
	refreshed := h.read(state)
	if got := attrs(t, refreshed)["description"]; !got.IsNull() {
		t.Fatalf("refreshed description = %s, want none", got)
	}

	h.applyAndConverge(refreshed, config)

	if got := api.project(t, "p1").Description; got == nil || *got != "Flags" {
		t.Fatalf("the API's description = %v", got)
	}
	if writes := api.writesSoFar(); len(writes) != 1 || writes[0] != `PATCH {"description":"Flags"}` {
		t.Fatalf("writes = %v", writes)
	}
}

// An empty description is kept as "" (it reads back as none from the API),
// and removing it then sends nothing: the API returns both alike.
func TestProjectEmptyDescriptionConverges(t *testing.T) {
	api := newFakeProjectsAPI()
	api.defaultOrg = "org-1"
	h := newProjectHarness(t, api)

	state := h.applyAndConverge(h.noResource(), h.config(map[string]string{"name": "My app", "slug": "my-app", "description": ""}))
	if got := attrs(t, state)["description"]; !got.Equal(str("")) {
		t.Fatalf("description = %s, want \"\"", got)
	}

	state = h.applyAndConverge(state, h.config(map[string]string{"name": "My app", "slug": "my-app"}))
	if got := attrs(t, state)["description"]; !got.IsNull() {
		t.Fatalf("description = %s, want none", got)
	}
	if writes := api.writesSoFar(); len(writes) != 1 || !strings.HasPrefix(writes[0], "POST ") {
		t.Fatalf("writes = %v, want only the create", writes)
	}
}
