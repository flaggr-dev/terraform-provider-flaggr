package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &AlertChannelResource{}
var _ resource.ResourceWithImportState = &AlertChannelResource{}

type AlertChannelResource struct {
	client *client.Client
}

type AlertChannelResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Type      types.String `tfsdk:"type"`
	Config    types.String `tfsdk:"config"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func NewAlertChannelResource() resource.Resource {
	return &AlertChannelResource{}
}

func (r *AlertChannelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_channel"
}

func (r *AlertChannelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr alert notification channel.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Alert channel ID.",
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
				Description: "Channel name.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "Channel type (email, slack, discord, pagerduty, webhook).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"config": schema.StringAttribute{
				Description: "Type-specific configuration as JSON.",
				Required:    true,
				Sensitive:   true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the channel is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
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

func (r *AlertChannelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AlertChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AlertChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var configMap map[string]interface{}
	if err := jsonUnmarshalString(plan.Config.ValueString(), &configMap); err != nil {
		resp.Diagnostics.AddError("Invalid config", "config must be valid JSON: "+err.Error())
		return
	}

	// A new channel has no stored secret for a placeholder to keep.
	if paths := misplacedPlaceholders(plan.Config.ValueString(), types.StringNull()); len(paths) > 0 {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Redacted secret in config", placeholderErrorDetail(paths))
		return
	}

	input := map[string]interface{}{
		"name":    plan.Name.ValueString(),
		"type":    plan.Type.ValueString(),
		"config":  configMap,
		"enabled": plan.Enabled.ValueBool(),
	}

	channel, err := r.client.CreateAlertChannel(ctx, plan.ProjectID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating alert channel", err.Error())
		return
	}

	plan.ID = types.StringValue(channel.ID)
	plan.CreatedAt = types.StringValue(channel.CreatedAt)
	plan.UpdatedAt = types.StringValue(channel.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AlertChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, channelID := parseCompositeIDPair(state.ID.ValueString(), state.ProjectID.ValueString())

	channel, err := r.client.GetAlertChannel(ctx, projectID, channelID)
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading alert channel", err.Error())
		return
	}

	state.Name = types.StringValue(channel.Name)
	state.Type = types.StringValue(channel.Type)
	state.Enabled = types.BoolValue(channel.Enabled)
	// Secrets come back as "[redacted]" below the admin tier: keep state's values for them.
	state.Config = configFromAPI(channel.Config, state.Config)
	state.ProjectID = types.StringValue(channel.ProjectID)
	state.CreatedAt = types.StringValue(channel.CreatedAt)
	state.UpdatedAt = types.StringValue(channel.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AlertChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AlertChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state AlertChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var configMap map[string]interface{}
	if err := jsonUnmarshalString(plan.Config.ValueString(), &configMap); err != nil {
		resp.Diagnostics.AddError("Invalid config", "config must be valid JSON: "+err.Error())
		return
	}

	input := map[string]interface{}{
		"name":    plan.Name.ValueString(),
		"config":  configMap,
		"enabled": plan.Enabled.ValueBool(),
	}

	// A placeholder may only go back where state holds the same one: the API
	// then keeps the stored secret.
	if paths := misplacedPlaceholders(plan.Config.ValueString(), state.Config); len(paths) > 0 {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Redacted secret in config", placeholderErrorDetail(paths))
		return
	}

	projectID, channelID := parseCompositeIDPair(state.ID.ValueString(), state.ProjectID.ValueString())

	channel, err := r.client.UpdateAlertChannel(ctx, projectID, channelID, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating alert channel", err.Error())
		return
	}

	plan.ID = state.ID
	plan.CreatedAt = types.StringValue(channel.CreatedAt)
	plan.UpdatedAt = types.StringValue(channel.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AlertChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, channelID := parseCompositeIDPair(state.ID.ValueString(), state.ProjectID.ValueString())

	err := r.client.DeleteAlertChannel(ctx, projectID, channelID)
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting alert channel", err.Error())
	}
}

func (r *AlertChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: projectId:channelId")
		return
	}

	channel, err := r.client.GetAlertChannel(ctx, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Error importing alert channel", err.Error())
		return
	}

	state := AlertChannelResourceModel{
		ID:        types.StringValue(channel.ID),
		ProjectID: types.StringValue(channel.ProjectID),
		Name:      types.StringValue(channel.Name),
		Type:      types.StringValue(channel.Type),
		Config:    types.StringValue(string(channel.Config)),
		Enabled:   types.BoolValue(channel.Enabled),
		CreatedAt: types.StringValue(channel.CreatedAt),
		UpdatedAt: types.StringValue(channel.UpdatedAt),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func parseCompositeIDPair(id, defaultProjectID string) (string, string) {
	if parts := strings.SplitN(id, ":", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return defaultProjectID, id
}
