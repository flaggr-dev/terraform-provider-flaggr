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
				Description: "URL-friendly identifier.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Organization description.",
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
	if org.Description != "" {
		state.Description = types.StringValue(org.Description)
	}
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
