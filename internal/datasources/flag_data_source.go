package datasources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/flaggr-dev/terraform-provider-flaggr/internal/flagkeys"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &FlagDataSource{}

type FlagDataSource struct {
	client *client.Client
}

type FlagDataSourceModel struct {
	Key          types.String `tfsdk:"key"`
	ServiceID    types.String `tfsdk:"service_id"`
	Environment  types.String `tfsdk:"environment"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Type         types.String `tfsdk:"type"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	DefaultValue types.String `tfsdk:"default_value"`
	ProjectID    types.String `tfsdk:"project_id"`
	IsPublic     types.Bool   `tfsdk:"is_public"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func NewFlagDataSource() datasource.DataSource {
	return &FlagDataSource{}
}

func (d *FlagDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flag"
}

func (d *FlagDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Flaggr feature flag by key, service, and environment.",
		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				Description: "Flag key.",
				Required:    true,
				// A reserved key's /api/flags/{key} URL is a built-in route, so no flag can be read by it.
				Validators: []validator.String{
					flagkeys.NotReserved(),
				},
			},
			"service_id": schema.StringAttribute{
				Description: "Service ID.",
				Required:    true,
			},
			"environment": schema.StringAttribute{
				Description: "Environment (development, staging, production).",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Flag name.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Flag description.",
				Computed:    true,
			},
			"type": schema.StringAttribute{
				Description: "Flag type.",
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the flag is enabled.",
				Computed:    true,
			},
			"default_value": schema.StringAttribute{
				Description: "Default value as JSON.",
				Computed:    true,
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID.",
				Computed:    true,
			},
			"is_public": schema.BoolAttribute{
				Description: "Whether the flag is publicly evaluable.",
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

func (d *FlagDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *FlagDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config FlagDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flag, err := d.client.GetFlag(ctx, config.Key.ValueString(), config.ServiceID.ValueString(), config.Environment.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading flag", err.Error())
		return
	}

	config.Name = types.StringValue(flag.Name)
	config.Type = types.StringValue(flag.Type)
	config.Enabled = types.BoolValue(flag.Enabled)
	config.ProjectID = types.StringValue(flag.ProjectID)
	config.IsPublic = types.BoolValue(flag.IsPublic)
	if flag.Description != "" {
		config.Description = types.StringValue(flag.Description)
	}

	defaultValueJSON, _ := json.Marshal(flag.DefaultValue)
	config.DefaultValue = types.StringValue(string(defaultValueJSON))
	config.CreatedAt = types.StringValue(flag.CreatedAt)
	config.UpdatedAt = types.StringValue(flag.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
