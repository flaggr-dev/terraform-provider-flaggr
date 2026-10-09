package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/flaggr-dev/terraform-provider-flaggr/internal/flagkeys"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &FlagResource{}
var _ resource.ResourceWithImportState = &FlagResource{}

type FlagResource struct {
	client *client.Client
}

type FlagResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Key          types.String `tfsdk:"key"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Type         types.String `tfsdk:"type"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	DefaultValue types.String `tfsdk:"default_value"`
	ProjectID    types.String `tfsdk:"project_id"`
	ServiceID    types.String `tfsdk:"service_id"`
	Environment  types.String `tfsdk:"environment"`
	IsPublic     types.Bool   `tfsdk:"is_public"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func NewFlagResource() resource.Resource {
	return &FlagResource{}
}

func (r *FlagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flag"
}

func (r *FlagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr feature flag.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite ID (key:serviceId:environment).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Description: "Flag key (unique within service+environment). The keys bulk, create-from-nl, evaluate, evaluate-debug, export, import and stream are reserved (built-in /api/flags routes).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					flagkeys.NotReserved(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-readable flag name.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Flag description.",
				Optional:    true,
			},
			"type": schema.StringAttribute{
				Description: "Flag type (boolean, string, number, object).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the flag is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"default_value": schema.StringAttribute{
				Description: "Default value as JSON (use jsonencode()).",
				Required:    true,
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_id": schema.StringAttribute{
				Description: "Service ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment": schema.StringAttribute{
				Description: "Environment (development, staging, production).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("production"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_public": schema.BoolAttribute{
				Description: "Whether the flag can be evaluated without authentication.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
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

func (r *FlagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func compositeID(key, serviceID, environment string) string {
	return key + ":" + serviceID + ":" + environment
}

func parseCompositeID(id string) (key, serviceID, environment string, err error) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid flag ID format, expected key:serviceId:environment, got: %s", id)
	}
	return parts[0], parts[1], parts[2], nil
}

func (r *FlagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var defaultValue interface{}
	if err := json.Unmarshal([]byte(plan.DefaultValue.ValueString()), &defaultValue); err != nil {
		resp.Diagnostics.AddError("Invalid default_value", "default_value must be valid JSON: "+err.Error())
		return
	}

	input := map[string]interface{}{
		"key":          plan.Key.ValueString(),
		"name":         plan.Name.ValueString(),
		"type":         plan.Type.ValueString(),
		"enabled":      plan.Enabled.ValueBool(),
		"defaultValue": defaultValue,
		"projectId":    plan.ProjectID.ValueString(),
		"serviceId":    plan.ServiceID.ValueString(),
		"environment":  plan.Environment.ValueString(),
		"isPublic":     plan.IsPublic.ValueBool(),
	}
	if !plan.Description.IsNull() {
		input["description"] = plan.Description.ValueString()
	}

	flag, err := r.client.CreateFlag(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating flag", err.Error())
		return
	}

	plan.ID = types.StringValue(compositeID(flag.Key, flag.ServiceID, flag.Environment))
	plan.CreatedAt = types.StringValue(flag.CreatedAt)
	plan.UpdatedAt = types.StringValue(flag.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FlagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, serviceID, environment, err := parseCompositeID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error parsing flag ID", err.Error())
		return
	}

	flag, err := r.client.GetFlag(ctx, key, serviceID, environment)
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading flag", err.Error())
		return
	}

	state.Key = types.StringValue(flag.Key)
	state.Name = types.StringValue(flag.Name)
	state.Type = types.StringValue(flag.Type)
	state.Enabled = types.BoolValue(flag.Enabled)
	state.ProjectID = types.StringValue(flag.ProjectID)
	state.ServiceID = types.StringValue(flag.ServiceID)
	state.Environment = types.StringValue(flag.Environment)
	state.IsPublic = types.BoolValue(flag.IsPublic)
	if flag.Description != "" {
		state.Description = types.StringValue(flag.Description)
	}

	defaultValueJSON, _ := json.Marshal(flag.DefaultValue)
	state.DefaultValue = types.StringValue(string(defaultValueJSON))
	state.CreatedAt = types.StringValue(flag.CreatedAt)
	state.UpdatedAt = types.StringValue(flag.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FlagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state FlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var defaultValue interface{}
	if err := json.Unmarshal([]byte(plan.DefaultValue.ValueString()), &defaultValue); err != nil {
		resp.Diagnostics.AddError("Invalid default_value", "default_value must be valid JSON: "+err.Error())
		return
	}

	// The flag is addressed by key, service and environment, as Read and Delete
	// do. None of them can change in place (each forces a replacement).
	key, serviceID, environment, err := parseCompositeID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error parsing flag ID", err.Error())
		return
	}

	input := map[string]interface{}{
		"name":         plan.Name.ValueString(),
		"enabled":      plan.Enabled.ValueBool(),
		"defaultValue": defaultValue,
		"isPublic":     plan.IsPublic.ValueBool(),
	}
	setUpdateDescription(input, plan.Description, state.Description)

	flag, err := r.client.UpdateFlag(ctx, key, serviceID, environment, input)
	if err != nil {
		// Returning without a new state keeps the flag's previous state, which
		// is still true: a change request changes nothing until it's applied.
		var changeRequest *client.ChangeRequestError
		if errors.As(err, &changeRequest) {
			resp.Diagnostics.AddError("Flag change needs approval", changeRequestDetail(changeRequest))
			return
		}
		resp.Diagnostics.AddError("Error updating flag", err.Error())
		return
	}

	plan.ID = state.ID
	plan.CreatedAt = types.StringValue(flag.CreatedAt)
	plan.UpdatedAt = types.StringValue(flag.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// changeRequestDetail explains a flag update that Flaggr turned into a change
// request: nothing changed, and running apply again opens another request.
func changeRequestDetail(err *client.ChangeRequestError) string {
	request := "a change request"
	if err.ChangeRequestID != "" {
		request = fmt.Sprintf("change request %s", err.ChangeRequestID)
	}
	return fmt.Sprintf(
		"The %s environment requires approval, so Flaggr didn't change the flag: it opened %s with this change instead. "+
			"Approve and apply the change request in Flaggr; the next terraform plan then shows no changes for this flag. "+
			"Running terraform apply again before that opens another change request.",
		err.Environment, request,
	)
}

func (r *FlagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, serviceID, environment, err := parseCompositeID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error parsing flag ID", err.Error())
		return
	}

	err = r.client.DeleteFlag(ctx, key, serviceID, environment)
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting flag", err.Error())
	}
}

func (r *FlagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	key, serviceID, environment, err := parseCompositeID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	flag, err := r.client.GetFlag(ctx, key, serviceID, environment)
	if err != nil {
		resp.Diagnostics.AddError("Error importing flag", err.Error())
		return
	}

	defaultValueJSON, _ := json.Marshal(flag.DefaultValue)

	state := FlagResourceModel{
		ID:           types.StringValue(compositeID(flag.Key, flag.ServiceID, flag.Environment)),
		Key:          types.StringValue(flag.Key),
		Name:         types.StringValue(flag.Name),
		Type:         types.StringValue(flag.Type),
		Enabled:      types.BoolValue(flag.Enabled),
		DefaultValue: types.StringValue(string(defaultValueJSON)),
		ProjectID:    types.StringValue(flag.ProjectID),
		ServiceID:    types.StringValue(flag.ServiceID),
		Environment:  types.StringValue(flag.Environment),
		IsPublic:     types.BoolValue(flag.IsPublic),
		CreatedAt:    types.StringValue(flag.CreatedAt),
		UpdatedAt:    types.StringValue(flag.UpdatedAt),
	}
	if flag.Description != "" {
		state.Description = types.StringValue(flag.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
