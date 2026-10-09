package resources_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// These tests drive flaggr_organization, flaggr_environment and flaggr_flag
// through the provider protocol the way project_convergence_test.go drives
// flaggr_project: plan, apply, refresh, plan again, against a fake Flaggr API
// that stores what it is sent. A change converges when the refresh after the
// apply reads back the state the apply wrote and the next plan is empty.

func attributePath(name string) *tftypes.AttributePath {
	return tftypes.NewAttributePath().WithAttributeName(name)
}

// --- organizations ---------------------------------------------------------

// apiOrganization is an organization as the fake API stores it.
type apiOrganization struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// fakeOrganizationsAPI serves /api/organizations like Flaggr: POST creates an
// organization with the slug it is sent, GET answers with what is stored, and
// PATCH changes the name and the description it is sent and nothing else. The
// route reads { name, description, settings } from the body, so a slug in it is
// ignored: nothing in Flaggr changes an organization's slug.
type fakeOrganizationsAPI struct {
	mu      sync.Mutex
	orgs    map[string]*apiOrganization
	writes  []string // "POST {...}", "PATCH {...}" and "DELETE <id>"
	updates int
}

func newFakeOrganizationsAPI(orgs ...apiOrganization) *fakeOrganizationsAPI {
	api := &fakeOrganizationsAPI{orgs: map[string]*apiOrganization{}}
	for i := range orgs {
		api.orgs[orgs[i].ID] = &orgs[i]
	}
	return api
}

func (f *fakeOrganizationsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(r.Body)
	reply := func(status int, v interface{}) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/organizations" {
		f.writes = append(f.writes, "POST "+string(body))
		var in apiOrganization
		if err := json.Unmarshal(body, &in); err != nil {
			reply(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		org := &apiOrganization{
			ID: fmt.Sprintf("org-%d", 100+len(f.orgs)), Name: strings.TrimSpace(in.Name), Slug: in.Slug,
			Description: strings.TrimSpace(in.Description), CreatedAt: "c", UpdatedAt: "u0",
		}
		f.orgs[org.ID] = org
		reply(http.StatusCreated, map[string]interface{}{"organization": org})
		return
	}

	org, ok := f.orgs[strings.TrimPrefix(r.URL.Path, "/api/organizations/")]
	if !ok {
		reply(http.StatusNotFound, map[string]string{"error": "Organization not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		reply(http.StatusOK, map[string]interface{}{"organization": org})
	case http.MethodDelete:
		f.writes = append(f.writes, "DELETE "+org.ID)
		delete(f.orgs, org.ID)
		reply(http.StatusOK, map[string]bool{"success": true})
	case http.MethodPatch:
		f.writes = append(f.writes, "PATCH "+string(body))
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			reply(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		for key, field := range map[string]*string{"name": &org.Name, "description": &org.Description} {
			if value, sent := raw[key]; sent {
				var s string
				if err := json.Unmarshal(value, &s); err == nil {
					*field = strings.TrimSpace(s)
				}
			}
		}
		f.updates++
		org.UpdatedAt = fmt.Sprintf("u%d", f.updates)
		reply(http.StatusOK, map[string]interface{}{"organization": org})
	default:
		reply(http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (f *fakeOrganizationsAPI) writesSoFar() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...)
}

func (f *fakeOrganizationsAPI) org(t *testing.T, id string) apiOrganization {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	org, ok := f.orgs[id]
	if !ok {
		t.Fatalf("organization %s isn't in the fake API", id)
	}
	return *org
}

func acme() apiOrganization {
	return apiOrganization{ID: "org-1", Name: "Acme", Slug: "acme", Description: "Widgets", CreatedAt: "c", UpdatedAt: "u0"}
}

// Flaggr can't change an organization's slug, so an update that did could only
// record a slug it never set: the apply reported success, and the next refresh
// read the old slug back, so every plan showed the change again. The plan
// refuses it instead, and nothing reaches Flaggr.
func TestOrganizationSlugChangeIsRefusedAtPlan(t *testing.T) {
	api := newFakeOrganizationsAPI(acme())
	h := newResourceHarness(t, "flaggr_organization", api)
	state := h.importResource("org-1")

	plan := h.plan(state, h.config(map[string]string{"name": "Acme", "slug": "acme-2", "description": "Widgets"}))
	if !hasError(plan.diags) {
		t.Fatalf("the plan changes the organization's slug: it plans\n%s\nover\n%s", plan.planned, state)
	}
	d := plan.diags[0]
	if d.Summary != "An organization's slug can't change" ||
		!d.Attribute.Equal(attributePath("slug")) ||
		!strings.Contains(d.Detail, `This organization's slug is "acme"`) ||
		!strings.Contains(d.Detail, `Set slug back to "acme", or create a new flaggr_organization with slug "acme-2"`) ||
		!strings.Contains(d.Detail, "run `terraform taint` on this resource, then apply") ||
		!strings.Contains(d.Detail, `to destroy the organization, set slug back to "acme" first`) {
		t.Fatalf("diagnostic = %s", diagnostics(plan.diags))
	}

	// Its own slug plans nothing, and a name change alongside is no excuse.
	h.planIsEmpty(state, h.config(map[string]string{"name": "Acme", "slug": "acme", "description": "Widgets"}))
	renamed := h.plan(state, h.config(map[string]string{"name": "Acme Inc", "slug": "acme-2", "description": "Widgets"}))
	if !hasError(renamed.diags) {
		t.Fatalf("the plan renames the organization and changes its slug: %s", renamed.planned)
	}

	if writes := api.writesSoFar(); len(writes) != 0 {
		t.Fatalf("writes = %v", writes)
	}
}

// The refusal is for an organization that exists: a new one takes any slug,
// and a change that leaves the slug alone goes through.
func TestOrganizationSlugIsFreeToChooseWhenCreatingAndKeptWhenUpdating(t *testing.T) {
	api := newFakeOrganizationsAPI()
	h := newResourceHarness(t, "flaggr_organization", api)

	created := h.applyAndConverge(h.noResource(), h.config(map[string]string{"name": "Acme", "slug": "acme-2", "description": "Widgets"}))
	if got := attrs(t, created)["slug"]; !got.Equal(str("acme-2")) {
		t.Fatalf("slug = %s", got)
	}

	renamed := h.applyAndConverge(created, h.config(map[string]string{"name": "Acme Inc", "slug": "acme-2", "description": "Widgets"}))
	id := attrs(t, renamed)["id"]
	var orgID string
	if err := id.As(&orgID); err != nil {
		t.Fatalf("id = %s: %v", id, err)
	}
	if got := api.org(t, orgID); got.Name != "Acme Inc" || got.Slug != "acme-2" {
		t.Fatalf("the organization in Flaggr = %+v", got)
	}
}

// --- environments ----------------------------------------------------------

// apiEnvironment is an environment as the fake API stores it.
type apiEnvironment struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"projectId"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Color       string          `json:"color,omitempty"`
	Order       int             `json:"order"`
	IsDefault   bool            `json:"isDefault"`
	Settings    map[string]bool `json:"settings,omitempty"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// fakeEnvironmentsAPI serves project p1's environments like Flaggr: GET lists
// them, POST creates one (with defaultColor when it is sent none, as for the
// environments Flaggr creates itself), and PATCH validates its body like
// updateEnvironmentSchema and changes only the fields it is sent. A color has
// to be a hex color: "" and null are refused, so no request clears one.
type fakeEnvironmentsAPI struct {
	mu           sync.Mutex
	envs         []*apiEnvironment
	defaultColor string
	writes       []string // "POST {...}", "PATCH {...}" and "DELETE <slug>"
	updates      int
}

func newFakeEnvironmentsAPI(envs ...apiEnvironment) *fakeEnvironmentsAPI {
	api := &fakeEnvironmentsAPI{}
	for i := range envs {
		api.envs = append(api.envs, &envs[i])
	}
	return api
}

func (f *fakeEnvironmentsAPI) find(slug string) *apiEnvironment {
	for _, env := range f.envs {
		if env.Slug == slug {
			return env
		}
	}
	return nil
}

func (f *fakeEnvironmentsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(r.Body)
	reply := func(status int, v interface{}) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	invalid := func(message string) {
		reply(http.StatusBadRequest, map[string]string{"error": "Validation Error", "message": message})
	}

	const base = "/api/projects/p1/environments"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			reply(http.StatusOK, map[string]interface{}{"environments": f.envs})
		case http.MethodPost:
			f.writes = append(f.writes, "POST "+string(body))
			var in struct {
				Slug        string          `json:"slug"`
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Color       *string         `json:"color"`
				Order       int             `json:"order"`
				Settings    map[string]bool `json:"settings"`
			}
			if err := json.Unmarshal(body, &in); err != nil {
				invalid("Invalid JSON")
				return
			}
			env := &apiEnvironment{
				ID: fmt.Sprintf("env-%d", 100+len(f.envs)), ProjectID: "p1", Slug: in.Slug, Name: in.Name,
				Description: in.Description, Color: f.defaultColor, Order: in.Order, Settings: in.Settings,
				CreatedAt: "c", UpdatedAt: "u0",
			}
			if in.Color != nil {
				if !hexColor.MatchString(*in.Color) {
					invalid("color: Color must be a hex color (e.g., #3b82f6)")
					return
				}
				env.Color = *in.Color
			}
			f.envs = append(f.envs, env)
			reply(http.StatusCreated, env)
		default:
			reply(http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		}
		return
	}

	env := f.find(strings.TrimPrefix(r.URL.Path, base+"/"))
	if env == nil {
		reply(http.StatusNotFound, map[string]string{"error": "Environment not found"})
		return
	}
	switch r.Method {
	case http.MethodDelete:
		f.writes = append(f.writes, "DELETE "+env.Slug)
		for i, e := range f.envs {
			if e == env {
				f.envs = append(f.envs[:i], f.envs[i+1:]...)
				break
			}
		}
		reply(http.StatusOK, map[string]bool{"success": true})
	case http.MethodPatch:
		f.writes = append(f.writes, "PATCH "+string(body))
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			invalid("Invalid JSON")
			return
		}
		var in struct {
			Name        *string         `json:"name"`
			Description *string         `json:"description"`
			Color       *string         `json:"color"`
			Order       *int            `json:"order"`
			Settings    map[string]bool `json:"settings"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			invalid("Invalid field type")
			return
		}
		for _, key := range []string{"name", "description", "color", "order"} {
			if value, sent := raw[key]; sent && string(value) == "null" {
				invalid(key + ": Expected a value, received null")
				return
			}
		}
		if in.Color != nil && !hexColor.MatchString(*in.Color) {
			invalid("color: Color must be a hex color")
			return
		}
		if in.Name == nil && in.Description == nil && in.Color == nil && in.Order == nil && len(in.Settings) == 0 {
			invalid("At least one field must be provided for update")
			return
		}
		if in.Name != nil {
			env.Name = *in.Name
		}
		if in.Description != nil {
			env.Description = *in.Description
		}
		if in.Color != nil {
			env.Color = *in.Color
		}
		if in.Order != nil {
			env.Order = *in.Order
		}
		if in.Settings != nil {
			if env.Settings == nil {
				env.Settings = map[string]bool{}
			}
			for key, value := range in.Settings {
				env.Settings[key] = value
			}
		}
		f.updates++
		env.UpdatedAt = fmt.Sprintf("u%d", f.updates)
		reply(http.StatusOK, env)
	default:
		reply(http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (f *fakeEnvironmentsAPI) writesSoFar() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...)
}

func (f *fakeEnvironmentsAPI) environment(t *testing.T, slug string) apiEnvironment {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	env := f.find(slug)
	if env == nil {
		t.Fatalf("environment %s isn't in the fake API", slug)
	}
	return *env
}

func qa() apiEnvironment {
	return apiEnvironment{
		ID: "env-1", ProjectID: "p1", Slug: "qa", Name: "QA", Color: "#10b981", Order: 30,
		Settings:  map[string]bool{"protectedEnvironment": false, "requireApproval": false},
		CreatedAt: "c", UpdatedAt: "u0",
	}
}

// Flaggr can't clear an environment's color (it only takes a hex color), so a
// color that isn't in the configuration is the one Flaggr has, not a removal to
// apply: before, the update left the color out, the refresh read it back, and
// every plan showed the removal again. A change to the environment that
// doesn't touch the color leaves it out of the request.
func TestEnvironmentWithoutAColorKeepsTheOneFlaggrHas(t *testing.T) {
	api := newFakeEnvironmentsAPI(qa())
	h := newResourceHarness(t, "flaggr_environment", api)
	state := h.importResource("p1:qa")
	if got := attrs(t, state)["color"]; !got.Equal(str("#10b981")) {
		t.Fatalf("color = %s", got)
	}

	// An environment set up with a color, then managed without one.
	h.planIsEmpty(state, h.config(map[string]string{"project_id": "p1", "slug": "qa", "name": "QA", "order": "30"}))

	renamed := h.applyAndConverge(state, h.config(map[string]string{"project_id": "p1", "slug": "qa", "name": "Quality", "order": "30"}))

	if got := attrs(t, renamed)["color"]; !got.Equal(str("#10b981")) {
		t.Fatalf("color = %s, want the one Flaggr has", got)
	}
	if got := api.environment(t, "qa"); got.Name != "Quality" || got.Color != "#10b981" {
		t.Fatalf("the environment in Flaggr = %+v", got)
	}
	writes := api.writesSoFar()
	if len(writes) != 1 || strings.Contains(writes[0], `"color"`) {
		t.Fatalf("writes = %v: the request names a color that doesn't change", writes)
	}
}

// A color in the configuration is sent when it changes, and the apply
// converges.
func TestEnvironmentColorChangeConverges(t *testing.T) {
	api := newFakeEnvironmentsAPI(qa())
	h := newResourceHarness(t, "flaggr_environment", api)
	config := h.config(map[string]string{"project_id": "p1", "slug": "qa", "name": "QA", "order": "30", "color": "#3b82f6"})

	state := h.applyAndConverge(h.importResource("p1:qa"), config)

	if got := attrs(t, state)["color"]; !got.Equal(str("#3b82f6")) {
		t.Fatalf("color = %s", got)
	}
	if got := api.environment(t, "qa"); got.Color != "#3b82f6" {
		t.Fatalf("the environment in Flaggr = %+v", got)
	}
	if writes := api.writesSoFar(); len(writes) != 1 || !strings.Contains(writes[0], `"color":"#3b82f6"`) {
		t.Fatalf("writes = %v", writes)
	}
}

// An environment created without a color has the one Flaggr gives it, or none:
// the state records that, not the plan's "known after apply" (which Terraform
// rejects after an apply), and the refresh reads back the same.
func TestEnvironmentCreatedWithoutAColorConverges(t *testing.T) {
	for name, defaultColor := range map[string]string{"Flaggr gives it none": "", "Flaggr gives it a color": "#6366f1"} {
		t.Run(name, func(t *testing.T) {
			api := newFakeEnvironmentsAPI()
			api.defaultColor = defaultColor
			h := newResourceHarness(t, "flaggr_environment", api)

			state := h.applyAndConverge(h.noResource(), h.config(map[string]string{"project_id": "p1", "slug": "qa", "name": "QA", "order": "30"}))

			want := nullString
			if defaultColor != "" {
				want = str(defaultColor)
			}
			if got := attrs(t, state)["color"]; !got.Equal(want) {
				t.Fatalf("color = %s, want %s", got, want)
			}
			if writes := api.writesSoFar(); len(writes) != 1 || strings.Contains(writes[0], `"color"`) {
				t.Fatalf("writes = %v: the create names a color that isn't configured", writes)
			}
		})
	}
}

// --- flags -----------------------------------------------------------------

// fakeApprovalFlagsAPI serves one flag, dark-mode in service svc-1's production
// environment, like Flaggr does for an environment that requires approval:
// GET answers with the stored flag, and PATCH opens a change request (202) and
// changes nothing. Approving a change request applies the changes it holds.
type fakeApprovalFlagsAPI struct {
	mu          sync.Mutex
	description string   // what Flaggr stores
	patches     []string // every PATCH body
	pending     []string // the bodies behind open change requests
}

func (f *fakeApprovalFlagsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	reply := func(status int, v interface{}) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	query := r.URL.Query()
	if r.URL.Path != "/api/flags/dark-mode" || query.Get("serviceId") != "svc-1" || query.Get("environment") != "production" {
		reply(http.StatusNotFound, map[string]string{"error": `Flag with key "dark-mode" not found`})
		return
	}
	switch r.Method {
	case http.MethodGet:
		reply(http.StatusOK, map[string]interface{}{
			"key": "dark-mode", "name": "Dark Mode", "description": f.description, "type": "boolean",
			"enabled": false, "defaultValue": false, "isPublic": false, "projectId": "p1", "serviceId": "svc-1",
			"environment": "production", "createdAt": "c", "updatedAt": "u0",
		})
	case http.MethodPatch:
		body, _ := io.ReadAll(r.Body)
		f.patches = append(f.patches, string(body))
		f.pending = append(f.pending, string(body))
		reply(http.StatusAccepted, map[string]interface{}{
			"message":       "This environment requires approval. A change request has been created.",
			"changeRequest": map[string]string{"id": fmt.Sprintf("cr-%d", len(f.pending)), "status": "pending"},
		})
	default:
		reply(http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// approve applies the open change requests, as approving them in Flaggr does.
func (f *fakeApprovalFlagsAPI) approve(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, body := range f.pending {
		var change struct {
			Description *string `json:"description"`
		}
		if err := json.Unmarshal([]byte(body), &change); err != nil {
			t.Fatalf("change request %s: %v", body, err)
		}
		if change.Description != nil {
			f.description = *change.Description
		}
	}
	f.pending = nil
}

func (f *fakeApprovalFlagsAPI) patchesSoFar() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.patches...)
}

// Removing a flag's description in an environment that requires approval opens
// a change request and fails the apply: the flag, and so the state, is as it
// was. Once someone approves the request Flaggr has no description, and the
// refresh has to see that: the plan then has nothing to do, as the apply's
// error says. Before, an empty description changed nothing in the state, the
// plan showed the removal again, and every apply opened another change request.
func TestFlagDescriptionRemovalApprovedInFlaggrConverges(t *testing.T) {
	api := &fakeApprovalFlagsAPI{description: "Old text"}
	h := newResourceHarness(t, "flaggr_flag", api)
	config := h.config(map[string]string{
		"key": "dark-mode", "name": "Dark Mode", "type": "boolean", "default_value": "false",
		"project_id": "p1", "service_id": "svc-1", "environment": "production",
	})
	state := h.importResource("dark-mode:svc-1:production")
	if got := attrs(t, state)["description"]; !got.Equal(str("Old text")) {
		t.Fatalf("description = %s", got)
	}

	plan := h.plan(state, config)
	if hasError(plan.diags) || plan.planned.Equal(state) {
		t.Fatalf("the plan doesn't remove the description: %s %s", plan.planned, diagnostics(plan.diags))
	}
	afterApply, diags := h.apply(state, config, plan)
	if !hasError(diags) || !strings.Contains(diagnostics(diags), "Flag change needs approval") {
		t.Fatalf("the apply opened a change request, but its diagnostics are %s", diagnostics(diags))
	}
	if !afterApply.Equal(state) {
		t.Fatalf("the state after the failed apply is\n%s\nbut the flag didn't change:\n%s", afterApply, state)
	}

	api.approve(t)

	refreshed := h.read(afterApply)
	if got := attrs(t, refreshed)["description"]; !got.IsNull() {
		t.Fatalf("description after the refresh = %s: Flaggr has none", got)
	}
	h.planIsEmpty(refreshed, config)
	if patches := api.patchesSoFar(); len(patches) != 1 || !strings.Contains(patches[0], `"description":""`) {
		t.Fatalf("PATCH requests = %v, want the one that removed the description", patches)
	}
}
