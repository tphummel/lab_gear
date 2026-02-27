package provider_test

import (
	"context"
	"testing"

	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/apiclient"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/provider"
)

// buildNullConfig constructs a tfsdk.Config where every attribute is null,
// which causes Configure to fall back on environment variables.
func buildNullConfig(t *testing.T, ctx context.Context, p fwprovider.Provider) tfsdk.Config {
	t.Helper()
	var schemaResp fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &schemaResp)

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	rawVal := tftypes.NewValue(schemaType, map[string]tftypes.Value{
		"endpoint": tftypes.NewValue(tftypes.String, nil), // null
		"token":    tftypes.NewValue(tftypes.String, nil), // null
	})
	return tfsdk.Config{Schema: schemaResp.Schema, Raw: rawVal}
}

// buildStringConfig constructs a tfsdk.Config with explicit string values.
func buildStringConfig(t *testing.T, ctx context.Context, p fwprovider.Provider, endpoint, token string) tfsdk.Config {
	t.Helper()
	var schemaResp fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &schemaResp)

	endpointVal := tftypes.NewValue(tftypes.String, nil)
	if endpoint != "" {
		endpointVal = tftypes.NewValue(tftypes.String, endpoint)
	}
	tokenVal := tftypes.NewValue(tftypes.String, nil)
	if token != "" {
		tokenVal = tftypes.NewValue(tftypes.String, token)
	}

	schemaType := schemaResp.Schema.Type().TerraformType(ctx)
	rawVal := tftypes.NewValue(schemaType, map[string]tftypes.Value{
		"endpoint": endpointVal,
		"token":    tokenVal,
	})
	return tfsdk.Config{Schema: schemaResp.Schema, Raw: rawVal}
}

// --- Metadata ---

func TestProvider_Metadata(t *testing.T) {
	ctx := context.Background()
	p := provider.New()
	var resp fwprovider.MetadataResponse
	p.Metadata(ctx, fwprovider.MetadataRequest{}, &resp)

	if resp.TypeName != "lab_gear" {
		t.Errorf("TypeName: got %q, want %q", resp.TypeName, "lab_gear")
	}
}

// --- Schema ---

func TestProvider_Schema_HasEndpoint(t *testing.T) {
	ctx := context.Background()
	p := provider.New()
	var resp fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &resp)

	if _, ok := resp.Schema.Attributes["endpoint"]; !ok {
		t.Error("provider schema missing 'endpoint' attribute")
	}
}

func TestProvider_Schema_HasToken(t *testing.T) {
	ctx := context.Background()
	p := provider.New()
	var resp fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &resp)

	if _, ok := resp.Schema.Attributes["token"]; !ok {
		t.Error("provider schema missing 'token' attribute")
	}
}

// --- Resources ---

func TestProvider_Resources_HasMachine(t *testing.T) {
	ctx := context.Background()
	p := provider.New()
	factories := p.Resources(ctx)
	if len(factories) != 1 {
		t.Errorf("Resources: got %d factories, want 1", len(factories))
	}
}

// --- DataSources ---

func TestProvider_DataSources_IsEmpty(t *testing.T) {
	ctx := context.Background()
	p := provider.New()
	sources := p.DataSources(ctx)
	if len(sources) != 0 {
		t.Errorf("DataSources: got %d, want 0", len(sources))
	}
}

// --- Configure: env vars ---

func TestProvider_Configure_WithEnvVars(t *testing.T) {
	t.Setenv("LAB_ENDPOINT", "http://lab.local:8080")
	t.Setenv("LAB_API_KEY", "secret-token")

	ctx := context.Background()
	p := provider.New()
	config := buildNullConfig(t, ctx, p)

	var resp fwprovider.ConfigureResponse
	p.Configure(ctx, fwprovider.ConfigureRequest{Config: config}, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with env vars: unexpected error: %v", resp.Diagnostics)
	}
	if resp.ResourceData == nil {
		t.Error("Configure: ResourceData should be set to *apiclient.Client")
	}
	if _, ok := resp.ResourceData.(*apiclient.Client); !ok {
		t.Errorf("Configure: ResourceData type: got %T, want *apiclient.Client", resp.ResourceData)
	}
}

// --- Configure: explicit config values ---

func TestProvider_Configure_WithConfigValues(t *testing.T) {
	// Clear env vars so config values take precedence
	t.Setenv("LAB_ENDPOINT", "")
	t.Setenv("LAB_API_KEY", "")

	ctx := context.Background()
	p := provider.New()
	config := buildStringConfig(t, ctx, p, "http://lab.local:9090", "config-token")

	var resp fwprovider.ConfigureResponse
	p.Configure(ctx, fwprovider.ConfigureRequest{Config: config}, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure with config values: unexpected error: %v", resp.Diagnostics)
	}
	if resp.ResourceData == nil {
		t.Error("Configure: ResourceData should be set")
	}
}

// --- Configure: config overrides env var ---

func TestProvider_Configure_ConfigOverridesEnvVar(t *testing.T) {
	t.Setenv("LAB_ENDPOINT", "http://env-value.local")
	t.Setenv("LAB_API_KEY", "env-token")

	ctx := context.Background()
	p := provider.New()
	// Config values take priority over env vars
	config := buildStringConfig(t, ctx, p, "http://config-value.local", "config-token")

	var resp fwprovider.ConfigureResponse
	p.Configure(ctx, fwprovider.ConfigureRequest{Config: config}, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure override: unexpected error: %v", resp.Diagnostics)
	}
}

// --- Configure: missing endpoint ---

func TestProvider_Configure_MissingEndpoint(t *testing.T) {
	t.Setenv("LAB_ENDPOINT", "")
	t.Setenv("LAB_API_KEY", "some-token")

	ctx := context.Background()
	p := provider.New()
	config := buildStringConfig(t, ctx, p, "", "some-token")

	var resp fwprovider.ConfigureResponse
	p.Configure(ctx, fwprovider.ConfigureRequest{Config: config}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure without endpoint: expected error, got none")
	}
}

// --- Configure: missing token ---

func TestProvider_Configure_MissingToken(t *testing.T) {
	t.Setenv("LAB_ENDPOINT", "http://lab.local")
	t.Setenv("LAB_API_KEY", "")

	ctx := context.Background()
	p := provider.New()
	config := buildStringConfig(t, ctx, p, "http://lab.local", "")

	var resp fwprovider.ConfigureResponse
	p.Configure(ctx, fwprovider.ConfigureRequest{Config: config}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure without token: expected error, got none")
	}
}

// --- Configure: both missing ---

func TestProvider_Configure_BothMissing(t *testing.T) {
	t.Setenv("LAB_ENDPOINT", "")
	t.Setenv("LAB_API_KEY", "")

	ctx := context.Background()
	p := provider.New()
	config := buildNullConfig(t, ctx, p)

	var resp fwprovider.ConfigureResponse
	p.Configure(ctx, fwprovider.ConfigureRequest{Config: config}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure with no endpoint or token: expected error, got none")
	}
}
