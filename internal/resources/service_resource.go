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

var _ resource.Resource = &ServiceResource{}
var _ resource.ResourceWithImportState = &ServiceResource{}

type ServiceResource struct {
	client *client.Client
}

type ServiceResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func NewServiceResource() resource.Resource {
	return &ServiceResource{}
}

func (r *ServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *ServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr service within a project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Service ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "ID of the project this service belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Service name.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly identifier.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Service description.",
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

func (r *ServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"name":      plan.Name.ValueString(),
		"projectId": plan.ProjectID.ValueString(),
	}
	if !plan.Description.IsNull() {
		input["description"] = plan.Description.ValueString()
	}

	service, err := r.client.CreateService(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating service", err.Error())
		return
	}

	plan.ID = types.StringValue(service.ID)
	plan.Slug = types.StringValue(service.Slug)
	plan.CreatedAt = types.StringValue(service.CreatedAt)
	plan.UpdatedAt = types.StringValue(service.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	service, err := r.client.GetService(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading service", err.Error())
		return
	}

	state.Name = types.StringValue(service.Name)
	state.Slug = types.StringValue(service.Slug)
	state.ProjectID = types.StringValue(service.ProjectID)
	if service.Description != "" {
		state.Description = types.StringValue(service.Description)
	}
	state.CreatedAt = types.StringValue(service.CreatedAt)
	state.UpdatedAt = types.StringValue(service.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"name": plan.Name.ValueString(),
	}
	setUpdateDescription(input, plan.Description, state.Description)

	service, err := r.client.UpdateService(ctx, state.ID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating service", err.Error())
		return
	}

	plan.ID = state.ID
	plan.Slug = types.StringValue(service.Slug)
	plan.CreatedAt = types.StringValue(service.CreatedAt)
	plan.UpdatedAt = types.StringValue(service.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteService(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting service", err.Error())
	}
}

func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	service, err := r.client.GetService(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing service", err.Error())
		return
	}

	state := ServiceResourceModel{
		ID:        types.StringValue(service.ID),
		ProjectID: types.StringValue(service.ProjectID),
		Name:      types.StringValue(service.Name),
		Slug:      types.StringValue(service.Slug),
		CreatedAt: types.StringValue(service.CreatedAt),
		UpdatedAt: types.StringValue(service.UpdatedAt),
	}
	if service.Description != "" {
		state.Description = types.StringValue(service.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
