package provider

import (
	"context"
	"os"
	"strings"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/flaggr-dev/terraform-provider-flaggr/internal/datasources"
	"github.com/flaggr-dev/terraform-provider-flaggr/internal/resources"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &FlaggrProvider{}

type FlaggrProvider struct {
	version string
}

type FlaggrProviderModel struct {
	APIURL   types.String `tfsdk:"api_url"`
	APIToken types.String `tfsdk:"api_token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FlaggrProvider{
			version: version,
		}
	}
}

func (p *FlaggrProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "flaggr"
	resp.Version = p.version
}

func (p *FlaggrProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing Flaggr feature flag resources.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Description: "The URL of the Flaggr API. Can also be set via FLAGGR_API_URL environment variable.",
				Optional:    true,
			},
			"api_token": schema.StringAttribute{
				Description: "Personal access token (fgp_…, recommended) or project API token (fgr_…). Can also be set via FLAGGR_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *FlaggrProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config FlaggrProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Trimmed: values pasted into env vars or tfvars often carry a trailing newline.
	apiURL := strings.TrimSpace(os.Getenv("FLAGGR_API_URL"))
	if !config.APIURL.IsNull() {
		apiURL = strings.TrimSpace(config.APIURL.ValueString())
	}
	if apiURL == "" {
		apiURL = "https://flaggr.dev"
	}

	apiToken := strings.TrimSpace(os.Getenv("FLAGGR_API_TOKEN"))
	if !config.APIToken.IsNull() {
		apiToken = strings.TrimSpace(config.APIToken.ValueString())
	}
	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"The provider requires a Flaggr personal access token. Set it via the api_token attribute or the FLAGGR_API_TOKEN environment variable.",
		)
		return
	}

	c := client.NewClient(apiURL, apiToken)
	c.UserAgent = "terraform-provider-flaggr/" + p.version
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *FlaggrProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewOrganizationResource,
		resources.NewOrganizationMemberResource,
		resources.NewProjectResource,
		resources.NewServiceResource,
		resources.NewFlagResource,
		resources.NewEnvironmentResource,
		resources.NewMetricSourceResource,
		resources.NewAlertChannelResource,
		resources.NewAlertRuleResource,
	}
}

func (p *FlaggrProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewOrganizationDataSource,
		datasources.NewProjectDataSource,
		datasources.NewServiceDataSource,
		datasources.NewFlagDataSource,
		datasources.NewEnvironmentDataSource,
	}
}
