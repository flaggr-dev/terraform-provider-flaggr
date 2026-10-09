package resources

import (
	"context"
	"fmt"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &OrganizationResource{}
var _ resource.ResourceWithImportState = &OrganizationResource{}

type OrganizationResource struct {
	client *client.Client
}

type OrganizationResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func NewOrganizationResource() resource.Resource {
	return &OrganizationResource{}
}

func (r *OrganizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *OrganizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Organization ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Organization name.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly identifier. Set when the organization is created: Flaggr can't change an organization's slug, so changing it on an existing organization is an error.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					organizationSlugCantChange{},
				},
			},
			"description": schema.StringAttribute{
				Description: "Organization description. Without it in the configuration, an apply clears the organization's description, including one set outside Terraform.",
				Optional:    true,
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

const organizationSlugChangeSummary = "An organization's slug can't change"

// organizationSlugChangeDetail explains a refused slug change: current is the
// organization's slug, planned the configured one. Flaggr has no way to change
// an organization's slug.
//
// The refusal comes from planning the configuration against the organization,
// and Terraform does that before it plans a replacement (`terraform apply
// -replace`) and in the refresh before `terraform destroy`, so both stop here
// while the configuration names another slug. A tainted organization is planned
// as a new one, which this doesn't check: that is the way to re-create the
// organization with the other slug.
func organizationSlugChangeDetail(current, planned string) string {
	return fmt.Sprintf("This organization's slug is %q, and Flaggr can't change the slug of an organization (its update API ignores one). "+
		"Set slug back to %q, or create a new flaggr_organization with slug %q.\n\n"+
		"To re-create this organization with slug %q instead (deleting this one), run `terraform taint` on this resource, then apply. "+
		"While slug is %q, this check also stops `terraform apply -replace` and `terraform destroy`: to destroy the organization, set slug back to %q first.",
		current, current, planned, planned, planned, current)
}

// organizationSlugCantChange refuses a plan that changes the slug of an
// existing organization. Nothing in Flaggr changes one (PATCH
// /api/organizations/{id} reads a name, a description and settings, and
// ignores the rest), so the update could only record a slug it never set, and
// the next refresh would put the real one back: the plan would never converge.
//
// It lets the plan through when there is nothing to compare: an organization
// being created or destroyed, or a slug not known until apply (Terraform plans
// again with it during the apply, and this runs again).
type organizationSlugCantChange struct{}

func (m organizationSlugCantChange) Description(_ context.Context) string {
	return "Refuses to change the slug of an existing organization: Flaggr can't change an organization's slug."
}

func (m organizationSlugCantChange) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m organizationSlugCantChange) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // creating or destroying
	}
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if req.PlanValue.Equal(req.StateValue) {
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path, organizationSlugChangeSummary,
		organizationSlugChangeDetail(req.StateValue.ValueString(), req.PlanValue.ValueString()))
}

func (r *OrganizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OrganizationResourceModel
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

	org, err := r.client.CreateOrganization(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization", err.Error())
		return
	}

	plan.ID = types.StringValue(org.ID)
	plan.CreatedAt = types.StringValue(org.CreatedAt)
	plan.UpdatedAt = types.StringValue(org.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OrganizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org, err := r.client.GetOrganization(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	state.Name = types.StringValue(org.Name)
	state.Slug = types.StringValue(org.Slug)
	state.Description = descriptionFromAPI(org.Description, state.Description)
	state.CreatedAt = types.StringValue(org.CreatedAt)
	state.UpdatedAt = types.StringValue(org.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OrganizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state OrganizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"name": plan.Name.ValueString(),
	}
	setUpdateDescription(input, plan.Description, state.Description)

	org, err := r.client.UpdateOrganization(ctx, state.ID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization", err.Error())
		return
	}

	plan.ID = state.ID
	plan.CreatedAt = types.StringValue(org.CreatedAt)
	plan.UpdatedAt = types.StringValue(org.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OrganizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteOrganization(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting organization", err.Error())
	}
}

func (r *OrganizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	org, err := r.client.GetOrganization(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing organization", err.Error())
		return
	}

	state := OrganizationResourceModel{
		ID:        types.StringValue(org.ID),
		Name:      types.StringValue(org.Name),
		Slug:      types.StringValue(org.Slug),
		CreatedAt: types.StringValue(org.CreatedAt),
		UpdatedAt: types.StringValue(org.UpdatedAt),
	}
	if org.Description != "" {
		state.Description = types.StringValue(org.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
