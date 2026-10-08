package resources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// fakeProjectAPI answers GET and PATCH /api/projects/p1 like Flaggr
// ({"project": {...}}) for a project in organization org-1, and records the
// bodies of the PATCH requests.
func fakeProjectAPI(t *testing.T, patches *[]string) *client.Client {
	t.Helper()
	var gets int
	return fakeProjectAPIIn(t, "org-1", &gets, patches)
}

// fakeProjectAPIIn is fakeProjectAPI for a project in organization org (""
// for none), counting the GET requests.
func fakeProjectAPIIn(t *testing.T, org string, gets *int, patches *[]string) *client.Client {
	t.Helper()
	orgID := ""
	if org != "" {
		orgID = fmt.Sprintf(`"orgId":%q,`, org)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/p1":
			*gets++
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"My app","slug":"my-app","description":"Flags",` + orgID + `"createdAt":"c","updatedAt":"u1"}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/projects/p1":
			body, _ := io.ReadAll(r.Body)
			*patches = append(*patches, string(body))
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"Renamed","slug":"my-app",` + orgID + `"createdAt":"c","updatedAt":"u2"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return client.NewClient(server.URL, "fgp_write")
}

func storedProject() ProjectResourceModel {
	return ProjectResourceModel{
		ID:             types.StringValue("p1"),
		Name:           types.StringValue("My app"),
		Slug:           types.StringValue("my-app"),
		Description:    types.StringValue("Flags"),
		OrganizationID: types.StringValue("org-1"),
		CreatedAt:      types.StringValue("c"),
		UpdatedAt:      types.StringValue("u1"),
	}
}

// plannedProject is the stored project as a plan sees it: the computed
// timestamps are unknown until the update returns them.
func plannedProject() ProjectResourceModel {
	planned := storedProject()
	planned.CreatedAt = types.StringUnknown()
	planned.UpdatedAt = types.StringUnknown()
	return planned
}

func projectUpdate(t *testing.T, r *ProjectResource, planned, prior ProjectResourceModel) (resource.UpdateResponse, ProjectResourceModel) {
	t.Helper()
	ctx := context.Background()
	s := schemaOf(t, r).Schema
	plan := tfsdk.Plan{Schema: s}
	state := tfsdk.State{Schema: s}
	if diags := plan.Set(ctx, &planned); diags.HasError() {
		t.Fatalf("plan: %v", diags)
	}
	if diags := state.Set(ctx, &prior); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s}}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)
	var got ProjectResourceModel
	if !resp.Diagnostics.HasError() {
		resp.State.Get(ctx, &got)
	}
	return resp, got
}

// CR-5: PATCH can't move a project to another organization, so an update
// must not record one (the next refresh would put org-1 back and the plan
// would never converge). The plan refuses it (projectOrganizationCantChange,
// ModifyPlan), including the plan Terraform makes again during the apply;
// this is Update's own check, so the state can't record it even if a plan got
// through.
func TestProjectUpdateRefusesAnOrganizationTheProjectIsNotIn(t *testing.T) {
	var patches []string
	r := &ProjectResource{client: fakeProjectAPI(t, &patches)}
	planned := plannedProject()
	planned.OrganizationID = types.StringValue("org-2")

	resp, _ := projectUpdate(t, r, planned, storedProject())
	if !resp.Diagnostics.HasError() {
		t.Fatal("an update into another organization succeeded")
	}
	diag := resp.Diagnostics.Errors()[0]
	if diag.Summary() != projectOrganizationMoveSummary || !strings.Contains(diag.Detail(), `"org-1"`) || !strings.Contains(diag.Detail(), `"org-2"`) {
		t.Fatalf("diagnostic = %q: %q", diag.Summary(), diag.Detail())
	}
	if len(patches) != 0 {
		t.Fatalf("PATCH sent: %v", patches)
	}
}

// An organization planned over none recorded (a state written without one,
// planned with -refresh=false) is recorded when it is the project's, with no
// PATCH: there is nothing the API stores to change.
func TestProjectUpdateRecordsTheProjectsOrganizationOverNone(t *testing.T) {
	var patches []string
	r := &ProjectResource{client: fakeProjectAPI(t, &patches)}
	prior := storedProject()
	prior.OrganizationID = types.StringNull()

	resp, got := projectUpdate(t, r, plannedProject(), prior)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	if len(patches) != 0 {
		t.Fatalf("PATCH sent: %v", patches)
	}
	if got.OrganizationID.ValueString() != "org-1" || got.CreatedAt.ValueString() != "c" || got.UpdatedAt.ValueString() != "u1" {
		t.Fatalf("state after update = %+v", got)
	}
}

// CR-5: a description removed from the configuration is cleared in Flaggr
// (the API keeps a field it isn't sent), so the state the update writes is
// what the next refresh reads.
func TestProjectUpdateClearsARemovedDescription(t *testing.T) {
	var patches []string
	r := &ProjectResource{client: fakeProjectAPI(t, &patches)}
	planned := plannedProject()
	planned.Description = types.StringNull()

	resp, got := projectUpdate(t, r, planned, storedProject())
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	if len(patches) != 1 || patches[0] != `{"description":""}` {
		t.Fatalf("patches = %v", patches)
	}
	if !got.Description.IsNull() || got.UpdatedAt.ValueString() != "u2" {
		t.Fatalf("state after update = %+v", got)
	}
}

// The API returns no description and an empty one alike, so a description
// going between them sends no PATCH (it would need the admin tier for
// nothing, and the API refuses an empty body).
func TestProjectUpdateSendsNothingForADescriptionBetweenNoneAndEmpty(t *testing.T) {
	for name, tc := range map[string]struct{ prior, planned types.String }{
		"none to empty": {types.StringNull(), types.StringValue("")},
		"empty to none": {types.StringValue(""), types.StringNull()},
	} {
		var patches []string
		r := &ProjectResource{client: fakeProjectAPI(t, &patches)}
		prior := storedProject()
		prior.Description = tc.prior
		planned := plannedProject()
		planned.Description = tc.planned

		resp, got := projectUpdate(t, r, planned, prior)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%s: update: %v", name, resp.Diagnostics)
		}
		if len(patches) != 0 {
			t.Fatalf("%s: PATCH sent: %v", name, patches)
		}
		if !got.Description.Equal(tc.planned) || got.UpdatedAt.ValueString() != "u1" {
			t.Fatalf("%s: state after update = %+v", name, got)
		}
	}
}

func TestProjectUpdateSendsOnlyTheChangedFields(t *testing.T) {
	var patches []string
	r := &ProjectResource{client: fakeProjectAPI(t, &patches)}
	planned := plannedProject()
	planned.Name = types.StringValue("Renamed")

	resp, got := projectUpdate(t, r, planned, storedProject())
	if resp.Diagnostics.HasError() {
		t.Fatalf("update: %v", resp.Diagnostics)
	}
	if len(patches) != 1 || patches[0] != `{"name":"Renamed"}` {
		t.Fatalf("patches = %v", patches)
	}
	if got.Name.ValueString() != "Renamed" || got.CreatedAt.ValueString() != "c" || got.UpdatedAt.ValueString() != "u2" {
		t.Fatalf("state after update = %+v", got)
	}
}

// CR-5: the plan refuses to move an existing project to another
// organization, and lets everything else through.
func TestProjectOrganizationCantChange(t *testing.T) {
	ctx := context.Background()
	s := schemaOf(t, &ProjectResource{}).Schema
	stored := tfsdk.State{Schema: s}
	project := storedProject()
	if diags := stored.Set(ctx, &project); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}
	planned := tfsdk.Plan{Schema: s, Raw: stored.Raw}
	null, unknown := types.StringNull(), types.StringUnknown()
	org := types.StringValue

	cases := []struct {
		name         string
		state        tfsdk.State
		plan         tfsdk.Plan
		prior, value types.String
		refused      bool
	}{
		{"move", stored, planned, org("org-1"), org("org-2"), true},
		{"unchanged", stored, planned, org("org-1"), org("org-1"), false},
		{"create", tfsdk.State{Schema: s}, planned, null, org("org-2"), false},
		{"none recorded", stored, planned, null, org("org-2"), false},
		{"known only at apply", stored, planned, org("org-1"), unknown, false},
		{"destroy", stored, tfsdk.Plan{Schema: s}, org("org-1"), null, false},
	}
	for _, tc := range cases {
		req := planmodifier.StringRequest{
			Path:       path.Root("organization_id"),
			State:      tc.state,
			StateValue: tc.prior,
			Plan:       tc.plan,
			PlanValue:  tc.value,
		}
		resp := planmodifier.StringResponse{PlanValue: tc.value}
		projectOrganizationCantChange{}.PlanModifyString(ctx, req, &resp)
		if resp.Diagnostics.HasError() != tc.refused {
			t.Fatalf("%s: diagnostics = %v, want refused = %v", tc.name, resp.Diagnostics, tc.refused)
		}
		if !resp.PlanValue.Equal(tc.value) {
			t.Fatalf("%s: plan value = %s, want it unchanged", tc.name, resp.PlanValue)
		}
		if tc.refused {
			diag := resp.Diagnostics.Errors()[0]
			if diag.Summary() != projectOrganizationMoveSummary || !strings.Contains(diag.Detail(), `back to "org-1"`) || !strings.Contains(diag.Detail(), `new flaggr_project in organization "org-2"`) {
				t.Fatalf("%s: diagnostic = %q: %q", tc.name, diag.Summary(), diag.Detail())
			}
		}
	}
}

// The API returns a project without a description and one cleared to "" as
// "": refresh keeps whichever of the two the state holds, and records a
// description cleared outside Terraform as none.
func TestDescriptionFromAPI(t *testing.T) {
	null, empty, flags := types.StringNull(), types.StringValue(""), types.StringValue("Flags")
	cases := []struct {
		name  string
		api   string
		prior types.String
		want  types.String
	}{
		{"set", "Flags", null, flags},
		{"changed", "New", flags, types.StringValue("New")},
		{"none", "", null, null},
		{"configured empty", "", empty, empty},
		{"cleared outside Terraform", "", flags, null},
	}
	for _, tc := range cases {
		if got := descriptionFromAPI(tc.api, tc.prior); !got.Equal(tc.want) {
			t.Errorf("%s: description = %s, want %s", tc.name, got, tc.want)
		}
	}
}

// Review of CR-5: Terraform plans the configuration against the project
// before a `terraform apply -replace` and in the refresh before `terraform
// destroy`, so the refusal stops both while organization_id names another
// organization. The diagnostic says so, and how to re-create the project in
// the other organization (taint it) or destroy it.
func TestProjectOrganizationMoveDetailSaysHowToRecreateOrDestroy(t *testing.T) {
	for _, tc := range []struct{ current, planned, destroy string }{
		{"org-1", "org-2", `to destroy the project, set organization_id back to "org-1" (or remove it) first.`},
		{"", "org-1", "to destroy the project, remove organization_id first."},
	} {
		detail := projectOrganizationMoveDetail(tc.current, tc.planned)
		for _, want := range []string{
			fmt.Sprintf("To re-create this project in organization %q instead (deleting this one), run `terraform taint` on this resource, then apply.", tc.planned),
			fmt.Sprintf("While organization_id is %q, this check also stops `terraform apply -replace` and `terraform destroy`", tc.planned),
			tc.destroy,
		} {
			if !strings.Contains(detail, want) {
				t.Errorf("detail for %q -> %q:\n%s\nwant it to contain %q", tc.current, tc.planned, detail, want)
			}
		}
	}
}

// projectModifyPlan runs ModifyPlan for a change from prior to planned (nil
// for none: a create, or a destroy).
func projectModifyPlan(t *testing.T, r *ProjectResource, prior, planned *ProjectResourceModel) resource.ModifyPlanResponse {
	t.Helper()
	ctx := context.Background()
	s := schemaOf(t, r).Schema
	state, plan := tfsdk.State{Schema: s}, tfsdk.Plan{Schema: s}
	if prior != nil {
		if diags := state.Set(ctx, prior); diags.HasError() {
			t.Fatalf("state: %v", diags)
		}
	}
	if planned != nil {
		if diags := plan.Set(ctx, planned); diags.HasError() {
			t.Fatalf("plan: %v", diags)
		}
	}
	resp := resource.ModifyPlanResponse{Plan: plan}
	r.ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, &resp)
	return resp
}

// Review of CR-5: with no organization recorded, projectOrganizationCantChange
// has nothing to compare, so ModifyPlan asks Flaggr and refuses at plan time an
// organization the project isn't in, instead of a plan every apply refuses
// (a project in no organization) or one that only the apply refuses (a state
// written without an organization, planned with -refresh=false). It asks
// nothing in any other plan.
func TestProjectModifyPlanChecksAnOrganizationOverNoneRecorded(t *testing.T) {
	recorded, noneRecorded := storedProject(), storedProject()
	noneRecorded.OrganizationID = types.StringNull()
	planned := func(org types.String) *ProjectResourceModel {
		p := plannedProject()
		p.OrganizationID = org
		return &p
	}
	org := types.StringValue

	cases := []struct {
		name           string
		projectOrg     string // the project's organization in Flaggr
		prior, planned *ProjectResourceModel
		gets           int
		refused        string // the start of the refusal's detail; "" for none
	}{
		{"project in no organization", "", &noneRecorded, planned(org("org-1")), 1, "This project doesn't belong to an organization"},
		{"another organization over none recorded", "org-1", &noneRecorded, planned(org("org-2")), 1, `This project belongs to organization "org-1"`},
		{"its own organization over none recorded", "org-1", &noneRecorded, planned(org("org-1")), 1, ""},
		{"none planned", "", &noneRecorded, planned(types.StringNull()), 0, ""},
		{"known only at apply", "", &noneRecorded, planned(types.StringUnknown()), 0, ""},
		{"recorded (the plan modifier compares it)", "org-1", &recorded, planned(org("org-2")), 0, ""},
		{"create", "org-1", nil, planned(org("org-2")), 0, ""},
		{"destroy", "", &noneRecorded, nil, 0, ""},
	}
	for _, tc := range cases {
		var gets int
		var patches []string
		r := &ProjectResource{client: fakeProjectAPIIn(t, tc.projectOrg, &gets, &patches)}

		resp := projectModifyPlan(t, r, tc.prior, tc.planned)
		if gets != tc.gets || len(patches) != 0 {
			t.Errorf("%s: %d GETs and PATCHes %v, want %d GETs", tc.name, gets, patches, tc.gets)
		}
		if tc.refused == "" {
			if resp.Diagnostics.HasError() {
				t.Errorf("%s: refused: %v", tc.name, resp.Diagnostics)
			}
			continue
		}
		if !resp.Diagnostics.HasError() {
			t.Errorf("%s: not refused", tc.name)
			continue
		}
		d := resp.Diagnostics.Errors()[0]
		withPath, ok := d.(diag.DiagnosticWithPath)
		if d.Summary() != projectOrganizationMoveSummary || !strings.HasPrefix(d.Detail(), tc.refused) || !ok || !withPath.Path().Equal(path.Root("organization_id")) {
			t.Errorf("%s: diagnostic = %q: %q", tc.name, d.Summary(), d.Detail())
		}
	}

	// A provider that isn't configured has no client to ask: nothing to do.
	if resp := projectModifyPlan(t, &ProjectResource{}, &noneRecorded, planned(org("org-1"))); resp.Diagnostics.HasError() {
		t.Errorf("unconfigured provider: %v", resp.Diagnostics)
	}
}
