package datasources

import (
	"context"
	"fmt"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &EnvironmentDataSource{}

type EnvironmentDataSource struct {
	client *client.Client
}

type EnvironmentDataSourceModel struct {
	ProjectID            types.String `tfsdk:"project_id"`
	Slug                 types.String `tfsdk:"slug"`
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	Color                types.String `tfsdk:"color"`
	Order                types.Int64  `tfsdk:"order"`
	IsDefault            types.Bool   `tfsdk:"is_default"`
	ProtectedEnvironment types.Bool   `tfsdk:"protected_environment"`
	RequireApproval      types.Bool   `tfsdk:"require_approval"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func NewEnvironmentDataSource() datasource.DataSource {
	return &EnvironmentDataSource{}
}

func (d *EnvironmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *EnvironmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a Flaggr project environment by project ID and slug.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Description: "Project ID.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "Environment slug.",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "Environment ID.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Display name.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Environment description.",
				Computed:    true,
			},
			"color": schema.StringAttribute{
				Description: "Hex color.",
				Computed:    true,
			},
			"order": schema.Int64Attribute{
				Description: "Pipeline order.",
				Computed:    true,
			},
			"is_default": schema.BoolAttribute{
				Description: "Whether this is a default environment.",
				Computed:    true,
			},
			"protected_environment": schema.BoolAttribute{
				Description: "Whether this is a protected environment.",
				Computed:    true,
			},
			"require_approval": schema.BoolAttribute{
				Description: "Whether promotions require approval.",
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

func (d *EnvironmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config EnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := d.client.GetEnvironment(ctx, config.ProjectID.ValueString(), config.Slug.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}

	config.ID = types.StringValue(env.ID)
	config.Name = types.StringValue(env.Name)
	config.Order = types.Int64Value(int64(env.Order))
	config.IsDefault = types.BoolValue(env.IsDefault)
	config.CreatedAt = types.StringValue(env.CreatedAt)
	config.UpdatedAt = types.StringValue(env.UpdatedAt)

	if env.Description != "" {
		config.Description = types.StringValue(env.Description)
	}
	if env.Color != "" {
		config.Color = types.StringValue(env.Color)
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
	config.ProtectedEnvironment = types.BoolValue(protected)
	config.RequireApproval = types.BoolValue(approval)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
