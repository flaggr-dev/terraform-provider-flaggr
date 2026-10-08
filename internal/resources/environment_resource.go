package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &EnvironmentResource{}
var _ resource.ResourceWithImportState = &EnvironmentResource{}

type EnvironmentResource struct {
	client *client.Client
}

type EnvironmentResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectID            types.String `tfsdk:"project_id"`
	Slug                 types.String `tfsdk:"slug"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	Color                types.String `tfsdk:"color"`
	Order                types.Int64  `tfsdk:"order"`
	ProtectedEnvironment types.Bool   `tfsdk:"protected_environment"`
	RequireApproval      types.Bool   `tfsdk:"require_approval"`
	IsDefault            types.Bool   `tfsdk:"is_default"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func NewEnvironmentResource() resource.Resource {
	return &EnvironmentResource{}
}

func (r *EnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *EnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr project environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Environment ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"slug": schema.StringAttribute{
				Description: "Environment slug (used as the environment value on flags).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Display name.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Environment description.",
				Optional:    true,
			},
			"color": schema.StringAttribute{
				Description: "Hex color for UI (e.g., #3b82f6).",
				Optional:    true,
			},
			"order": schema.Int64Attribute{
				Description: "Pipeline order (lower = earlier in pipeline).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(50),
			},
			"protected_environment": schema.BoolAttribute{
				Description: "Whether this is a protected environment.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"require_approval": schema.BoolAttribute{
				Description: "Whether promotions to this environment require approval.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"is_default": schema.BoolAttribute{
				Description: "Whether this is a default environment (read-only).",
				Computed:    true,
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

func (r *EnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"slug":  plan.Slug.ValueString(),
		"name":  plan.Name.ValueString(),
		"order": plan.Order.ValueInt64(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input["description"] = plan.Description.ValueString()
	}
	if !plan.Color.IsNull() && !plan.Color.IsUnknown() {
		input["color"] = plan.Color.ValueString()
	}

	settings := map[string]interface{}{}
	if plan.ProtectedEnvironment.ValueBool() {
		settings["protectedEnvironment"] = true
	}
	if plan.RequireApproval.ValueBool() {
		settings["requireApproval"] = true
	}
	if len(settings) > 0 {
		input["settings"] = settings
	}

	env, err := r.client.CreateEnvironment(ctx, plan.ProjectID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating environment", err.Error())
		return
	}

	plan.ID = types.StringValue(env.ID)
	plan.IsDefault = types.BoolValue(env.IsDefault)
	plan.CreatedAt = types.StringValue(env.CreatedAt)
	plan.UpdatedAt = types.StringValue(env.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	slug := state.Slug.ValueString()

	env, err := r.client.GetEnvironment(ctx, projectID, slug)
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}

	state.ID = types.StringValue(env.ID)
	state.Name = types.StringValue(env.Name)
	state.Order = types.Int64Value(int64(env.Order))
	state.IsDefault = types.BoolValue(env.IsDefault)
	state.CreatedAt = types.StringValue(env.CreatedAt)
	state.UpdatedAt = types.StringValue(env.UpdatedAt)

	if env.Description != "" {
		state.Description = types.StringValue(env.Description)
	}
	if env.Color != "" {
		state.Color = types.StringValue(env.Color)
	}

	protected := false
	approval := false
	if env.Settings != nil {
		if env.Settings.ProtectedEnvironment != nil {
			protected = *env.Settings.ProtectedEnvironment
		}
		if env.Settings.RequireApproval != nil {
			approval = *env.Settings.RequireApproval
		}
	}
	state.ProtectedEnvironment = types.BoolValue(protected)
	state.RequireApproval = types.BoolValue(approval)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"name":  plan.Name.ValueString(),
		"order": plan.Order.ValueInt64(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		input["description"] = plan.Description.ValueString()
	}
	if !plan.Color.IsNull() && !plan.Color.IsUnknown() {
		input["color"] = plan.Color.ValueString()
	}

	settings := map[string]interface{}{
		"protectedEnvironment": plan.ProtectedEnvironment.ValueBool(),
		"requireApproval":      plan.RequireApproval.ValueBool(),
	}
	input["settings"] = settings

	env, err := r.client.UpdateEnvironment(ctx, plan.ProjectID.ValueString(), plan.Slug.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating environment", err.Error())
		return
	}

	plan.ID = types.StringValue(env.ID)
	plan.IsDefault = types.BoolValue(env.IsDefault)
	plan.CreatedAt = types.StringValue(env.CreatedAt)
	plan.UpdatedAt = types.StringValue(env.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteEnvironment(ctx, state.ProjectID.ValueString(), state.Slug.ValueString())
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting environment", err.Error())
	}
}

func (r *EnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: projectId:slug
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: projectId:slug")
		return
	}

	env, err := r.client.GetEnvironment(ctx, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Error importing environment", err.Error())
		return
	}

	protected := false
	approval := false
	if env.Settings != nil {
		if env.Settings.ProtectedEnvironment != nil {
			protected = *env.Settings.ProtectedEnvironment
		}
		if env.Settings.RequireApproval != nil {
			approval = *env.Settings.RequireApproval
		}
	}

	state := EnvironmentResourceModel{
		ID:                   types.StringValue(env.ID),
		ProjectID:            types.StringValue(env.ProjectID),
		Slug:                 types.StringValue(env.Slug),
		Name:                 types.StringValue(env.Name),
		Order:                types.Int64Value(int64(env.Order)),
		IsDefault:            types.BoolValue(env.IsDefault),
		ProtectedEnvironment: types.BoolValue(protected),
		RequireApproval:      types.BoolValue(approval),
		CreatedAt:            types.StringValue(env.CreatedAt),
		UpdatedAt:            types.StringValue(env.UpdatedAt),
	}
	if env.Description != "" {
		state.Description = types.StringValue(env.Description)
	}
	if env.Color != "" {
		state.Color = types.StringValue(env.Color)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
