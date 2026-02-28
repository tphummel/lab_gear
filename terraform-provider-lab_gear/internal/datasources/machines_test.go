package datasources_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/apiclient"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/datasources"
)

// testMachinesModel mirrors machinesDataSourceModel for reading state in tests.
type testMachinesModel struct {
	Kind     types.String      `tfsdk:"kind"`
	Machines []testMachineItem `tfsdk:"machines"`
}

type testMachineItem struct {
	ID        types.String  `tfsdk:"id"`
	Name      types.String  `tfsdk:"name"`
	Kind      types.String  `tfsdk:"kind"`
	Make      types.String  `tfsdk:"make"`
	Model     types.String  `tfsdk:"model"`
	CPU       types.String  `tfsdk:"cpu"`
	RAMGB     types.Int64   `tfsdk:"ram_gb"`
	StorageTB types.Float64 `tfsdk:"storage_tb"`
	Location  types.String  `tfsdk:"location"`
	Serial    types.String  `tfsdk:"serial"`
	Notes     types.String  `tfsdk:"notes"`
}

// getDataSourceSchema returns the schema from the data source.
func getDataSourceSchema(t *testing.T, d datasource.DataSource) datasource.SchemaResponse {
	t.Helper()
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	return resp
}

// newMockServer creates an httptest server and a client pointing at it.
func newMockServer(t *testing.T, handler http.HandlerFunc) *apiclient.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return apiclient.NewClient(srv.URL, "test-token")
}

// configureDataSource injects the client into the data source.
func configureDataSource(t *testing.T, d datasource.DataSource, client *apiclient.Client) {
	t.Helper()
	dc, ok := d.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("data source does not implement DataSourceWithConfigure")
	}
	var resp datasource.ConfigureResponse
	dc.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}
}

// buildConfig constructs a tfsdk.Config for the machines data source.
// kind may be "" to represent a null/omitted filter.
func buildConfig(t *testing.T, schm datasource.SchemaResponse, kind string) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	tfType := schm.Schema.Type().TerraformType(ctx)
	objType := tfType.(tftypes.Object)
	machinesType := objType.AttributeTypes["machines"]

	var kindVal tftypes.Value
	if kind == "" {
		kindVal = tftypes.NewValue(tftypes.String, nil) // null
	} else {
		kindVal = tftypes.NewValue(tftypes.String, kind)
	}

	raw := tftypes.NewValue(tfType, map[string]tftypes.Value{
		"kind":     kindVal,
		"machines": tftypes.NewValue(machinesType, nil), // null — computed
	})
	return tfsdk.Config{Schema: schm.Schema, Raw: raw}
}

// --- Metadata ---

func TestMachinesDataSource_Metadata(t *testing.T) {
	d := datasources.NewMachinesDataSource()
	var resp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "lab_gear"}, &resp)

	want := "lab_gear_machines"
	if resp.TypeName != want {
		t.Errorf("TypeName: got %q, want %q", resp.TypeName, want)
	}
}

// --- Schema ---

func TestMachinesDataSource_Schema_HasKindAttribute(t *testing.T) {
	d := datasources.NewMachinesDataSource()
	schm := getDataSourceSchema(t, d)

	attr, ok := schm.Schema.Attributes["kind"]
	if !ok {
		t.Fatal("schema missing 'kind' attribute")
	}
	if !attr.IsOptional() {
		t.Error("'kind' attribute should be Optional")
	}
}

func TestMachinesDataSource_Schema_HasMachinesAttribute(t *testing.T) {
	d := datasources.NewMachinesDataSource()
	schm := getDataSourceSchema(t, d)

	attr, ok := schm.Schema.Attributes["machines"]
	if !ok {
		t.Fatal("schema missing 'machines' attribute")
	}
	if !attr.IsComputed() {
		t.Error("'machines' attribute should be Computed")
	}
}

// --- Configure ---

func TestMachinesDataSource_Configure_NilData(t *testing.T) {
	d := datasources.NewMachinesDataSource()
	dc := d.(datasource.DataSourceWithConfigure)

	var resp datasource.ConfigureResponse
	dc.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: nil}, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure(nil): unexpected error: %v", resp.Diagnostics)
	}
}

func TestMachinesDataSource_Configure_WrongType(t *testing.T) {
	d := datasources.NewMachinesDataSource()
	dc := d.(datasource.DataSourceWithConfigure)

	var resp datasource.ConfigureResponse
	dc.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "not-a-client"}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure(wrong type): expected error, got none")
	}
}

func TestMachinesDataSource_Configure_ValidClient(t *testing.T) {
	d := datasources.NewMachinesDataSource()
	dc := d.(datasource.DataSourceWithConfigure)

	client := apiclient.NewClient("http://localhost", "token")
	var resp datasource.ConfigureResponse
	dc.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: client}, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure(valid client): unexpected error: %v", resp.Diagnostics)
	}
}

// --- Read ---

func TestMachinesDataSource_Read_ReturnsList(t *testing.T) {
	ctx := context.Background()
	d := datasources.NewMachinesDataSource()
	schm := getDataSourceSchema(t, d)

	apiMachines := []apiclient.Machine{
		{ID: "uuid-1", Name: "pve1", Kind: "proxmox", Make: "Dell", Model: "R640"},
		{ID: "uuid-2", Name: "nas01", Kind: "nas", Make: "Synology", Model: "DS920+"},
	}
	client := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/machines" {
			t.Errorf("path: got %q, want /api/v1/machines", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(apiMachines)
	})
	configureDataSource(t, d, client)

	config := buildConfig(t, schm, "")
	stateRaw := tftypes.NewValue(schm.Schema.Type().TerraformType(ctx), nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schm.Schema, Raw: stateRaw}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: unexpected error: %v", resp.Diagnostics)
	}

	var state testMachinesModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("Read: state.Get: %v", diags)
	}
	if len(state.Machines) != 2 {
		t.Fatalf("machines count: got %d, want 2", len(state.Machines))
	}
	if state.Machines[0].Name.ValueString() != "pve1" {
		t.Errorf("machines[0].Name: got %q, want %q", state.Machines[0].Name.ValueString(), "pve1")
	}
	if state.Machines[1].Kind.ValueString() != "nas" {
		t.Errorf("machines[1].Kind: got %q, want %q", state.Machines[1].Kind.ValueString(), "nas")
	}
}

func TestMachinesDataSource_Read_WithKindFilter(t *testing.T) {
	ctx := context.Background()
	d := datasources.NewMachinesDataSource()
	schm := getDataSourceSchema(t, d)

	client := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "kind=proxmox" {
			t.Errorf("query: got %q, want kind=proxmox", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]apiclient.Machine{
			{ID: "uuid-1", Name: "pve1", Kind: "proxmox", Make: "Dell", Model: "R640"},
		})
	})
	configureDataSource(t, d, client)

	config := buildConfig(t, schm, "proxmox")
	stateRaw := tftypes.NewValue(schm.Schema.Type().TerraformType(ctx), nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schm.Schema, Raw: stateRaw}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: unexpected error: %v", resp.Diagnostics)
	}

	var state testMachinesModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("state.Get: %v", diags)
	}
	if len(state.Machines) != 1 {
		t.Fatalf("machines count: got %d, want 1", len(state.Machines))
	}
	if state.Kind.ValueString() != "proxmox" {
		t.Errorf("Kind: got %q, want proxmox", state.Kind.ValueString())
	}
}

func TestMachinesDataSource_Read_EmptyList(t *testing.T) {
	ctx := context.Background()
	d := datasources.NewMachinesDataSource()
	schm := getDataSourceSchema(t, d)

	client := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]apiclient.Machine{})
	})
	configureDataSource(t, d, client)

	config := buildConfig(t, schm, "")
	stateRaw := tftypes.NewValue(schm.Schema.Type().TerraformType(ctx), nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schm.Schema, Raw: stateRaw}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: unexpected error: %v", resp.Diagnostics)
	}

	var state testMachinesModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("state.Get: %v", diags)
	}
	if len(state.Machines) != 0 {
		t.Errorf("expected 0 machines, got %d", len(state.Machines))
	}
}

func TestMachinesDataSource_Read_APIError(t *testing.T) {
	ctx := context.Background()
	d := datasources.NewMachinesDataSource()
	schm := getDataSourceSchema(t, d)

	client := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	configureDataSource(t, d, client)

	config := buildConfig(t, schm, "")
	stateRaw := tftypes.NewValue(schm.Schema.Type().TerraformType(ctx), nil)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schm.Schema, Raw: stateRaw}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Read: expected error on API failure, got none")
	}
}
