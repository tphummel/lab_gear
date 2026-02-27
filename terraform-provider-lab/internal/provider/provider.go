package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tomflanagan/terraform-provider-lab/internal/config"
	"github.com/tomflanagan/terraform-provider-lab/internal/labapi"
	"github.com/tomflanagan/terraform-provider-lab/internal/resources"
)

var _ provider.Provider = &LabProvider{}

type LabProvider struct {
	version string
}

type LabProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New() provider.Provider {
	return &LabProvider{version: "dev"}
}

func (p *LabProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "lab"
	resp.Version = p.version
}

func (p *LabProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "API base URL for lab-assets. Can also be set via LAB_ENDPOINT.",
			},
			"api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Bearer token for API auth. Can also be set via LAB_API_KEY.",
			},
		},
	}
}

func (p *LabProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data LabProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configEndpoint := ""
	if !data.Endpoint.IsNull() && !data.Endpoint.IsUnknown() {
		configEndpoint = data.Endpoint.ValueString()
	}

	configAPIKey := ""
	if !data.APIKey.IsNull() && !data.APIKey.IsUnknown() {
		configAPIKey = data.APIKey.ValueString()
	}

	endpoint, apiKey := config.ResolveProviderConfig(
		configEndpoint,
		configAPIKey,
		os.Getenv("LAB_ENDPOINT"),
		os.Getenv("LAB_API_KEY"),
	)

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing lab API endpoint",
			"Set provider attribute endpoint or LAB_ENDPOINT environment variable.",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing lab API key",
			"Set provider attribute api_key or LAB_API_KEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := labapi.NewClient(endpoint, apiKey)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create lab API client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *LabProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewMachineResource,
	}
}

func (p *LabProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
