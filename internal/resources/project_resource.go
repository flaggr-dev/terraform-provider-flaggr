package resources

import (
	"context"
	"fmt"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}
var _ resource.ResourceWithModifyPlan = &ProjectResource{}

type ProjectResource struct {
	client *client.Client
}

type ProjectResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Slug           types.String `tfsdk:"slug"`
	Description    types.String `tfsdk:"description"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

func (r *ProjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Project ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Project name.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly identifier.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Project description. Without it in the configuration, an apply clears the project's description, including one set outside Terraform.",
				Optional:    true,
			},
			"organization_id": schema.StringAttribute{
				Description: "Organization ID this project belongs to. If omitted, defaults to the user's sole organization. " +
					"Set when the project is created: a project can't move to another organization, so changing it on an existing project is an error.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					projectOrganizationCantChange{},
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Creation timestamp.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Last update timestamp.",
				Computed:    true,
			},
		},
	}
}

func (r *ProjectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"name": plan.Name.ValueString(),
		"slug": plan.Slug.ValueString(),
	}
	if !plan.Description.IsNull() {
		input["description"] = plan.Description.ValueString()
	}
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() {
		input["orgId"] = plan.OrganizationID.ValueString()
	}

	project, err := r.client.CreateProject(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	plan.ID = types.StringValue(project.ID)
	switch {
	case project.OrganizationID != "":
		plan.OrganizationID = types.StringValue(project.OrganizationID)
	case plan.OrganizationID.IsUnknown():
		// An account in no organization creates its projects outside any;
		// the state can't hold the plan's "known after apply".
		plan.OrganizationID = types.StringNull()
	}
	plan.CreatedAt = types.StringValue(project.CreatedAt)
	plan.UpdatedAt = types.StringValue(project.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.GetProject(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	state.Name = types.StringValue(project.Name)
	state.Slug = types.StringValue(project.Slug)
	state.Description = descriptionFromAPI(project.Description, state.Description)
	if project.OrganizationID != "" {
		state.OrganizationID = types.StringValue(project.OrganizationID)
	}
	state.CreatedAt = types.StringValue(project.CreatedAt)
	state.UpdatedAt = types.StringValue(project.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID

	// No API moves a project to another organization or adds one to an
	// organization. The plan already refuses an organization the project
	// isn't in (projectOrganizationCantChange against a recorded one,
	// ModifyPlan against the project's own when none is recorded), and
	// Terraform plans again during the apply, so this only reaches a new
	// organization over none recorded that is the project's own: a state
	// written without one, planned with -refresh=false (a refresh records the
	// project's organization). Checking it again keeps the state from ever
	// recording an organization the project isn't in.
	if org := plan.OrganizationID; !org.IsNull() && !org.IsUnknown() && !org.Equal(state.OrganizationID) {
		resp.Diagnostics.Append(r.checkProjectOrganization(ctx, state.ID.ValueString(), org.ValueString())...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	input := projectUpdateInput(plan, state)
	if len(input) == 0 {
		// Nothing the API stores differs (the organization checked above, or
		// a description going between none and "", which the API returns
		// alike): skip the PATCH, which needs the admin tier and refuses an
		// empty body.
		plan.CreatedAt = state.CreatedAt
		plan.UpdatedAt = state.UpdatedAt
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	project, err := r.client.UpdateProject(ctx, state.ID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	plan.CreatedAt = types.StringValue(project.CreatedAt)
	plan.UpdatedAt = types.StringValue(project.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// projectUpdateInput is the PATCH body for an update: only the fields that
// changed. The slug in particular: Flaggr lets only project owners change a
// slug, so sending an unchanged one would make every update by a non-owner
// (or a credential that can't change slugs) fail. A description removed from
// the configuration is cleared by sending "": the API keeps a field it isn't
// sent and refuses null. An empty body means there is nothing to send
// (Update then skips the PATCH).
func projectUpdateInput(plan, state ProjectResourceModel) map[string]interface{} {
	input := map[string]interface{}{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		input["name"] = plan.Name.ValueString()
	}
	if plan.Slug.ValueString() != state.Slug.ValueString() {
		input["slug"] = plan.Slug.ValueString()
	}
	// ValueString is "" for null, so a description going between none and ""
	// (stored and returned alike) sends nothing.
	if !plan.Description.IsUnknown() && plan.Description.ValueString() != state.Description.ValueString() {
		input["description"] = plan.Description.ValueString()
	}
	return input
}

// descriptionFromAPI is the state's description for the one the API returns.
// The API returns a project without a description and one cleared to "" the
// same way, as "": that keeps a prior null or "" (a configuration that omits
// the description or sets it to ""), and turns a non-empty prior into null —
// the description was cleared outside Terraform, which the next plan shows.
func descriptionFromAPI(api string, prior types.String) types.String {
	if api != "" {
		return types.StringValue(api)
	}
	if prior.IsUnknown() || prior.ValueString() != "" {
		return types.StringNull()
	}
	return prior
}

const projectOrganizationMoveSummary = "A project can't move to another organization"

// projectOrganizationMoveDetail explains a refused organization_id change:
// current is the project's organization ("" for none), planned the configured
// one. Flaggr has no API or dashboard action that moves a project, or adds an
// existing one to an organization.
//
// The refusal comes from planning the configuration against the project, and
// Terraform does that before it plans a replacement (`terraform apply
// -replace`) and in the refresh before `terraform destroy`, so both stop here
// while the configuration names another organization. A tainted project is
// planned as a new one, which this doesn't check: that is the way to
// re-create the project in the other organization.
func projectOrganizationMoveDetail(current, planned string) string {
	detail := fmt.Sprintf("This project belongs to organization %q, and Flaggr can't move a project to another organization. "+
		"Set organization_id back to %q (or remove it from the configuration), or create a new flaggr_project in organization %q.",
		current, current, planned)
	keep := fmt.Sprintf("set organization_id back to %q (or remove it)", current)
	if current == "" {
		detail = fmt.Sprintf("This project doesn't belong to an organization, and Flaggr can't add an existing project to one. "+
			"Remove organization_id from the configuration, or create a new flaggr_project in organization %q.", planned)
		keep = "remove organization_id"
	}
	return detail + fmt.Sprintf("\n\nTo re-create this project in organization %q instead (deleting this one), "+
		"run `terraform taint` on this resource, then apply. While organization_id is %q, this check also stops "+
		"`terraform apply -replace` and `terraform destroy`: to destroy the project, %s first.", planned, planned, keep)
}

// projectOrganizationCantChange refuses a plan that moves an existing project
// to another organization. Nothing in Flaggr moves a project (PATCH
// /api/projects/{id} ignores orgId), so the update could only record a change
// it never made, and the next refresh would put the real organization back:
// the plan would never converge.
//
// It lets the plan through when there is nothing to compare: a project being
// created, no organization recorded in the state (ModifyPlan then checks the
// project's own), or a value not known until apply (Terraform plans again with
// it during the apply, and this runs again).
type projectOrganizationCantChange struct{}

func (m projectOrganizationCantChange) Description(_ context.Context) string {
	return "Refuses to change the organization of an existing project: a project can't move to another organization."
}

func (m projectOrganizationCantChange) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m projectOrganizationCantChange) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // creating or destroying
	}
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if req.PlanValue.Equal(req.StateValue) {
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path, projectOrganizationMoveSummary,
		projectOrganizationMoveDetail(req.StateValue.ValueString(), req.PlanValue.ValueString()))
}

// ModifyPlan refuses, at plan time, an organization planned over none
// recorded that isn't the project's own; projectOrganizationCantChange only
// compares a recorded one. A refresh records the project's organization, so
// none is recorded for a project in no organization (Flaggr can't add an
// existing project to one: every apply would fail and every plan show the
// change again) or for a state written without one and planned with
// -refresh=false. With nothing to compare it asks Flaggr, and it makes no
// request in any other plan.
func (r *ProjectResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || r.client == nil {
		return // creating, destroying, or no configured provider to ask
	}
	var id, recorded, planned types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("organization_id"), &recorded)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("organization_id"), &planned)...)
	if resp.Diagnostics.HasError() || !recorded.IsNull() || planned.IsNull() || planned.IsUnknown() {
		return
	}
	resp.Diagnostics.Append(r.checkProjectOrganization(ctx, id.ValueString(), planned.ValueString())...)
}

// checkProjectOrganization refuses an organization the project isn't in.
func (r *ProjectResource) checkProjectOrganization(ctx context.Context, id, organization string) diag.Diagnostics {
	var diags diag.Diagnostics
	project, err := r.client.GetProject(ctx, id)
	if err != nil {
		diags.AddError("Error reading project", err.Error())
		return diags
	}
	if project.OrganizationID != organization {
		diags.AddAttributeError(path.Root("organization_id"), projectOrganizationMoveSummary,
			projectOrganizationMoveDetail(project.OrganizationID, organization))
	}
	return diags
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteProject(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
	}
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, err := r.client.GetProject(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing project", err.Error())
		return
	}

	state := ProjectResourceModel{
		ID:        types.StringValue(project.ID),
		Name:      types.StringValue(project.Name),
		Slug:      types.StringValue(project.Slug),
		CreatedAt: types.StringValue(project.CreatedAt),
		UpdatedAt: types.StringValue(project.UpdatedAt),
	}
	if project.Description != "" {
		state.Description = types.StringValue(project.Description)
	}
	if project.OrganizationID != "" {
		state.OrganizationID = types.StringValue(project.OrganizationID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
