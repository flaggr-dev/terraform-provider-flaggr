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

var _ resource.Resource = &MetricSourceResource{}
var _ resource.ResourceWithImportState = &MetricSourceResource{}

type MetricSourceResource struct {
	client *client.Client
}

type MetricSourceResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Type      types.String `tfsdk:"type"`
	Config    types.String `tfsdk:"config"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func NewMetricSourceResource() resource.Resource {
	return &MetricSourceResource{}
}

func (r *MetricSourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_metric_source"
}

func (r *MetricSourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Flaggr metric source for golden signal monitoring.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Metric source ID.",
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
				Description: "Metric source name.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "Metric source type (prometheus, datadog, cloudwatch, custom_webhook, internal, signalfx, newrelic, grafana_cloud).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"config": schema.StringAttribute{
				Description: "Type-specific configuration as JSON (use jsonencode()).",
				Required:    true,
				Sensitive:   true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the metric source is enabled.",
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

func (r *MetricSourceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MetricSourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MetricSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"name":    plan.Name.ValueString(),
		"type":    plan.Type.ValueString(),
		"enabled": plan.Enabled.ValueBool(),
	}

	// Parse config JSON
	var configMap map[string]interface{}
	if err := jsonUnmarshalString(plan.Config.ValueString(), &configMap); err != nil {
		resp.Diagnostics.AddError("Invalid config", "config must be valid JSON: "+err.Error())
		return
	}
	// A new source has no stored secret for a placeholder to keep.
	if paths := misplacedPlaceholders(plan.Config.ValueString(), types.StringNull()); len(paths) > 0 {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Redacted secret in config", placeholderErrorDetail(paths))
		return
	}
	configMap["type"] = plan.Type.ValueString()
	input["config"] = configMap

	source, err := r.client.CreateMetricSource(ctx, plan.ProjectID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating metric source", err.Error())
		return
	}

	plan.ID = types.StringValue(source.ID)
	plan.CreatedAt = types.StringValue(source.CreatedAt)
	plan.UpdatedAt = types.StringValue(source.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MetricSourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MetricSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, sourceID := parseMetricSourceID(state.ID.ValueString(), state.ProjectID.ValueString())

	source, err := r.client.GetMetricSource(ctx, projectID, sourceID)
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading metric source", err.Error())
		return
	}

	state.Name = types.StringValue(source.Name)
	state.Type = types.StringValue(source.Type)
	state.Enabled = types.BoolValue(source.Enabled)
	// Secrets come back as "[redacted]" below the admin tier: keep state's
	// values for them, and ignore the "type" key the provider adds itself.
	state.Config = configFromAPI(source.Config, state.Config, "type")
	state.ProjectID = types.StringValue(source.ProjectID)
	state.CreatedAt = types.StringValue(source.CreatedAt)
	state.UpdatedAt = types.StringValue(source.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MetricSourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MetricSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state MetricSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"name":    plan.Name.ValueString(),
		"enabled": plan.Enabled.ValueBool(),
	}

	var configMap map[string]interface{}
	if err := jsonUnmarshalString(plan.Config.ValueString(), &configMap); err != nil {
		resp.Diagnostics.AddError("Invalid config", "config must be valid JSON: "+err.Error())
		return
	}
	// A placeholder may only go back where state holds the same one: the API
	// then keeps the stored secret.
	if paths := misplacedPlaceholders(plan.Config.ValueString(), state.Config); len(paths) > 0 {
		resp.Diagnostics.AddAttributeError(path.Root("config"), "Redacted secret in config", placeholderErrorDetail(paths))
		return
	}
	configMap["type"] = plan.Type.ValueString()
	input["config"] = configMap

	projectID, sourceID := parseMetricSourceID(state.ID.ValueString(), state.ProjectID.ValueString())

	source, err := r.client.UpdateMetricSource(ctx, projectID, sourceID, input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating metric source", err.Error())
		return
	}

	plan.ID = state.ID
	plan.CreatedAt = types.StringValue(source.CreatedAt)
	plan.UpdatedAt = types.StringValue(source.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MetricSourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MetricSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, sourceID := parseMetricSourceID(state.ID.ValueString(), state.ProjectID.ValueString())

	err := r.client.DeleteMetricSource(ctx, projectID, sourceID)
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting metric source", err.Error())
	}
}

func (r *MetricSourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: projectId:sourceId
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: projectId:sourceId")
		return
	}

	source, err := r.client.GetMetricSource(ctx, parts[0], parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Error importing metric source", err.Error())
		return
	}

	state := MetricSourceResourceModel{
		ID:        types.StringValue(source.ID),
		ProjectID: types.StringValue(source.ProjectID),
		Name:      types.StringValue(source.Name),
		Type:      types.StringValue(source.Type),
		Config:    types.StringValue(string(source.Config)),
		Enabled:   types.BoolValue(source.Enabled),
		CreatedAt: types.StringValue(source.CreatedAt),
		UpdatedAt: types.StringValue(source.UpdatedAt),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func parseMetricSourceID(id, projectID string) (string, string) {
	// If the ID contains ":", it's a composite ID from import
	if parts := strings.SplitN(id, ":", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	// Otherwise, ID is just the source ID and projectID comes from state
	return projectID, id
}
