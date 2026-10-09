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

var _ resource.Resource = &AlertRuleResource{}
var _ resource.ResourceWithImportState = &AlertRuleResource{}

type AlertRuleResource struct {
	client *client.Client
}

type AlertRuleResourceModel struct {
	ID              types.String  `tfsdk:"id"`
	ProjectID       types.String  `tfsdk:"project_id"`
	Name            types.String  `tfsdk:"name"`
	Description     types.String  `tfsdk:"description"`
	Severity        types.String  `tfsdk:"severity"`
	ConditionType   types.String  `tfsdk:"condition_type"`
	Threshold       types.Float64 `tfsdk:"threshold"`
	WindowMinutes   types.Int64   `tfsdk:"window_minutes"`
	Channels        types.String  `tfsdk:"channels"`
	Enabled         types.Bool    `tfsdk:"enabled"`
	CooldownMinutes types.Int64   `tfsdk:"cooldown_minutes"`
	CreatedAt       types.String  `tfsdk:"created_at"`
	UpdatedAt       types.String  `tfsdk:"updated_at"`
}

func NewAlertRuleResource() resource.Resource {
	return &AlertRuleResource{}
}

func (r *AlertRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_rule"
}

func (r *AlertRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr alert rule. Enabled rules are evaluated every 5 minutes against stored evaluation metrics and audit events; a breach opens an incident and notifies the rule's channels.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Alert rule ID.",
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
			"name": schema.StringAttribute{
				Description: "Rule name.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Rule description. Without it in the configuration, an apply clears the rule's description, including one set outside Terraform.",
				Optional:    true,
			},
			"severity": schema.StringAttribute{
				Description: "Alert severity (critical, warning, info).",
				Required:    true,
			},
			"condition_type": schema.StringAttribute{
				Description: "Condition type: error_rate (percent, 1 = 1%), p95_latency (ms), evaluation_count (fires above the threshold), toggle_drift (error-rate rise in percentage points after a toggle) or auto_rollback (automated rollbacks in the window). grpc_errors and cache_hit_rate are no longer evaluated and are rejected for new rules (existing rules keep them).",
				Required:    true,
			},
			"threshold": schema.Float64Attribute{
				Description: "Threshold value; the rule fires when the condition's value rises above it (error_rate in percent, 1 = 1%).",
				Required:    true,
			},
			"window_minutes": schema.Int64Attribute{
				Description: "Time window in minutes for evaluating the condition (evaluated as at least 5 minutes, 15 for toggle_drift and auto_rollback).",
				Required:    true,
			},
			"channels": schema.StringAttribute{
				Description: "Alert channels as JSON array of {channelId, channelName} objects.",
				Required:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the rule is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"cooldown_minutes": schema.Int64Attribute{
				Description: "Minimum minutes between incidents for this rule; an open incident is not re-notified.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(15),
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

func (r *AlertRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AlertRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AlertRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var channels []interface{}
	if err := jsonUnmarshalString(plan.Channels.ValueString(), &channels); err != nil {
		resp.Diagnostics.AddError("Invalid channels", "channels must be valid JSON array: "+err.Error())
		return
	}

	input := map[string]interface{}{
		"name":            plan.Name.ValueString(),
		"severity":        plan.Severity.ValueString(),
		"conditionType":   plan.ConditionType.ValueString(),
		"threshold":       plan.Threshold.ValueFloat64(),
		"windowMinutes":   plan.WindowMinutes.ValueInt64(),
		"channels":        channels,
		"enabled":         plan.Enabled.ValueBool(),
		"cooldownMinutes": plan.CooldownMinutes.ValueInt64(),
	}
	if !plan.Description.IsNull() {
		input["description"] = plan.Description.ValueString()
	}

	rule, err := r.client.CreateAlertRule(ctx, plan.ProjectID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating alert rule", err.Error())
		return
	}

	plan.ID = types.StringValue(rule.ID)
	plan.CreatedAt = types.StringValue(rule.CreatedAt)
	plan.UpdatedAt = types.StringValue(rule.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AlertRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, ruleID := parseCompositeIDPair(state.ID.ValueString(), state.ProjectID.ValueString())

	rule, err := r.client.GetAlertRule(ctx, projectID, ruleID)
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading alert rule", err.Error())
		return
	}

	state.Name = types.StringValue(rule.Name)
	state.Severity = types.StringValue(rule.Severity)
	state.ConditionType = types.StringValue(rule.ConditionType)
	state.Threshold = types.Float64Value(rule.Threshold)
	state.WindowMinutes = types.Int64Value(int64(rule.WindowMinutes))
	state.Enabled = types.BoolValue(rule.Enabled)
	state.CooldownMinutes = types.Int64Value(int64(rule.CooldownMinutes))
	state.ProjectID = types.StringValue(rule.ProjectID)
	state.CreatedAt = types.StringValue(rule.CreatedAt)
	state.UpdatedAt = types.StringValue(rule.UpdatedAt)
	state.Description = descriptionFromAPI(rule.Description, state.Description)

	// Serialize channels back to JSON
	channelsJSON, _ := jsonMarshal(rule.Channels)
	state.Channels = types.StringValue(string(channelsJSON))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AlertRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AlertRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state AlertRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var channels []interface{}
	if err := jsonUnmarshalString(plan.Channels.ValueString(), &channels); err != nil {
		resp.Diagnostics.AddError("Invalid channels", "channels must be valid JSON array: "+err.Error())
		return
	}

	input := map[string]interface{}{
		"name":            plan.Name.ValueString(),
		"severity":        plan.Severity.ValueString(),
		"conditionType":   plan.ConditionType.ValueString(),
		"threshold":       plan.Threshold.ValueFloat64(),
		"windowMinutes":   plan.WindowMinutes.ValueInt64(),
		"channels":        channels,
		"enabled":         plan.Enabled.ValueBool(),
		"cooldownMinutes": plan.CooldownMinutes.ValueInt64(),
	}
	setUpdateDescription(input, plan.Description, state.Description)

	projectID, ruleID := parseCompositeIDPair(state.ID.ValueString(), state.ProjectID.ValueString())

	rule, err := r.client.UpdateAlertRule(ctx, projectID, ruleID, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating alert rule", err.Error())
		return
	}

	plan.ID = state.ID
	plan.CreatedAt = types.StringValue(rule.CreatedAt)
	plan.UpdatedAt = types.StringValue(rule.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AlertRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, ruleID := parseCompositeIDPair(state.ID.ValueString(), state.ProjectID.ValueString())

	err := r.client.DeleteAlertRule(ctx, projectID, ruleID)
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting alert rule", err.Error())
	}
}

func (r *AlertRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: projectId:ruleId")
		return
	}

	rule, err := r.client.GetAlertRule(ctx, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Error importing alert rule", err.Error())
		return
	}

	channelsJSON, _ := jsonMarshal(rule.Channels)

	state := AlertRuleResourceModel{
		ID:              types.StringValue(rule.ID),
		ProjectID:       types.StringValue(rule.ProjectID),
		Name:            types.StringValue(rule.Name),
		Severity:        types.StringValue(rule.Severity),
		ConditionType:   types.StringValue(rule.ConditionType),
		Threshold:       types.Float64Value(rule.Threshold),
		WindowMinutes:   types.Int64Value(int64(rule.WindowMinutes)),
		Channels:        types.StringValue(string(channelsJSON)),
		Enabled:         types.BoolValue(rule.Enabled),
		CooldownMinutes: types.Int64Value(int64(rule.CooldownMinutes)),
		CreatedAt:       types.StringValue(rule.CreatedAt),
		UpdatedAt:       types.StringValue(rule.UpdatedAt),
	}
	if rule.Description != "" {
		state.Description = types.StringValue(rule.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
