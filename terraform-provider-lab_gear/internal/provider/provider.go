package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/resources"
)

// New returns the provider factory function expected by providerserver.Serve.
func New() provider.Provider {
	return &labGearProvider{}
}

type labGearProvider struct{}

func (p *labGearProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "lab_gear"
}

func (p *labGearProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages physical machine inventory in the lab_gear service.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "Base URL of the lab_gear API (e.g. https://assets.lab.local). " +
					"Can also be set via the LAB_ENDPOINT environment variable.",
				Optional: true,
			},
			"token": schema.StringAttribute{
				Description: "Bearer token for lab_gear API authentication. " +
					"Can also be set via the LAB_API_KEY environment variable.",
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

type labGearProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

func (p *labGearProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config labGearProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := os.Getenv("LAB_ENDPOINT")
	if !config.Endpoint.IsNull() && !config.Endpoint.IsUnknown() {
		endpoint = config.Endpoint.ValueString()
	}
	if endpoint == "" {
		resp.Diagnostics.AddError("Missing endpoint",
			"endpoint must be set in the provider configuration or via the LAB_ENDPOINT environment variable.")
		return
	}

	token := os.Getenv("LAB_API_KEY")
	if !config.Token.IsNull() && !config.Token.IsUnknown() {
		token = config.Token.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddError("Missing token",
			"token must be set in the provider configuration or via the LAB_API_KEY environment variable.")
		return
	}

	client := NewClient(endpoint, token)
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *labGearProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewMachineResource,
	}
}

func (p *labGearProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
