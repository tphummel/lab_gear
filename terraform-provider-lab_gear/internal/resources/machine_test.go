package resources_test

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/apiclient"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/resources"
)

// testMachineModel mirrors machineModel for decoding state in tests.
type testMachineModel struct {
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

// getSchema retrieves the machine resource schema.
func getSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	ctx := context.Background()
	var resp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resp)
	return resp.Schema
}

// buildPlan constructs a tfsdk.Plan for the machine schema. Required fields
// (name, kind, make, model) must be supplied; optional computed fields
// default to Unknown so the provider can fill them in.
func buildPlan(t *testing.T, schm resourceschema.Schema, name, kind, make, model string) tfsdk.Plan {
	t.Helper()
	ctx := context.Background()
	schemaType := schm.Type().TerraformType(ctx)
	raw := tftypes.NewValue(schemaType, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"name":       tftypes.NewValue(tftypes.String, name),
		"kind":       tftypes.NewValue(tftypes.String, kind),
		"make":       tftypes.NewValue(tftypes.String, make),
		"model":      tftypes.NewValue(tftypes.String, model),
		"cpu":        tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"ram_gb":     tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
		"storage_tb": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
		"location":   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"serial":     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"notes":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	return tfsdk.Plan{Schema: schm, Raw: raw}
}

// buildState constructs a tfsdk.State populated with a known machine.
func buildState(t *testing.T, schm resourceschema.Schema, m apiclient.Machine) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	schemaType := schm.Type().TerraformType(ctx)
	raw := tftypes.NewValue(schemaType, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, m.ID),
		"name":       tftypes.NewValue(tftypes.String, m.Name),
		"kind":       tftypes.NewValue(tftypes.String, m.Kind),
		"make":       tftypes.NewValue(tftypes.String, m.Make),
		"model":      tftypes.NewValue(tftypes.String, m.Model),
		"cpu":        tftypes.NewValue(tftypes.String, m.CPU),
		"ram_gb":     tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(m.RAMGB)),
		"storage_tb": tftypes.NewValue(tftypes.Number, big.NewFloat(m.StorageTB)),
		"location":   tftypes.NewValue(tftypes.String, m.Location),
		"serial":     tftypes.NewValue(tftypes.String, m.Serial),
		"notes":      tftypes.NewValue(tftypes.String, m.Notes),
	})
	return tfsdk.State{Schema: schm, Raw: raw}
}

// emptyState returns a null-initialised state with the schema set.
func emptyState(schm resourceschema.Schema) tfsdk.State {
	ctx := context.Background()
	return tfsdk.State{
		Schema: schm,
		Raw:    tftypes.NewValue(schm.Type().TerraformType(ctx), nil),
	}
}

// newMockServer spins up an httptest.Server and returns the client pointing at it.
func newMockServer(t *testing.T, handler http.HandlerFunc) *apiclient.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return apiclient.NewClient(srv.URL, "test-token")
}

// configureResource injects client into the resource; fails the test on error.
func configureResource(t *testing.T, r resource.Resource, client *apiclient.Client) {
	t.Helper()
	ctx := context.Background()
	rc, ok := r.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("resource does not implement ResourceWithConfigure")
	}
	var resp resource.ConfigureResponse
	rc.Configure(ctx, resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}
}

// writeMachine encodes m as JSON with statusCode.
func writeMachine(w http.ResponseWriter, statusCode int, m apiclient.Machine) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(m)
}

// --- Metadata ---

func TestMachineResource_Metadata(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "lab_gear"}, &resp)

	want := "lab_gear_machine"
	if resp.TypeName != want {
		t.Errorf("TypeName: got %q, want %q", resp.TypeName, want)
	}
}

// --- Schema ---

func TestMachineResource_Schema_RequiredFields(t *testing.T) {
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	required := []string{"name", "kind", "make", "model"}
	for _, attr := range required {
		a, ok := schm.Attributes[attr]
		if !ok {
			t.Errorf("schema missing required attribute %q", attr)
			continue
		}
		if !a.IsRequired() {
			t.Errorf("attribute %q should be Required", attr)
		}
	}
}

func TestMachineResource_Schema_ComputedFields(t *testing.T) {
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	computed := []string{"id", "cpu", "ram_gb", "storage_tb", "location", "serial", "notes"}
	for _, attr := range computed {
		a, ok := schm.Attributes[attr]
		if !ok {
			t.Errorf("schema missing computed attribute %q", attr)
			continue
		}
		if !a.IsComputed() {
			t.Errorf("attribute %q should be Computed", attr)
		}
	}
}

// --- Configure ---

func TestMachineResource_Configure_NilData(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	rc := r.(resource.ResourceWithConfigure)

	var resp resource.ConfigureResponse
	rc.Configure(ctx, resource.ConfigureRequest{ProviderData: nil}, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure(nil): unexpected error: %v", resp.Diagnostics)
	}
}

func TestMachineResource_Configure_WrongType(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	rc := r.(resource.ResourceWithConfigure)

	var resp resource.ConfigureResponse
	rc.Configure(ctx, resource.ConfigureRequest{ProviderData: "not-a-client"}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Configure(wrong type): expected error, got none")
	}
}

func TestMachineResource_Configure_ValidClient(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	rc := r.(resource.ResourceWithConfigure)

	client := apiclient.NewClient("http://localhost", "token")
	var resp resource.ConfigureResponse
	rc.Configure(ctx, resource.ConfigureRequest{ProviderData: client}, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Configure(valid client): unexpected error: %v", resp.Diagnostics)
	}
}

// --- Create ---

func TestMachineResource_Create_Success(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	apiMachine := apiclient.Machine{
		ID:    "uuid-create-1",
		Name:  "pve1",
		Kind:  "proxmox",
		Make:  "Dell",
		Model: "R640",
	}
	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Errorf("Create: method: got %q, want POST", req.Method)
		}
		writeMachine(w, http.StatusCreated, apiMachine)
	})
	configureResource(t, r, client)

	plan := buildPlan(t, schm, "pve1", "proxmox", "Dell", "R640")
	resp := &resource.CreateResponse{State: emptyState(schm)}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: unexpected error: %v", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("Create: state should not be null after successful create")
	}

	var state testMachineModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("Create: state.Get: %v", diags)
	}
	if state.ID.ValueString() != apiMachine.ID {
		t.Errorf("ID: got %q, want %q", state.ID.ValueString(), apiMachine.ID)
	}
	if state.Name.ValueString() != apiMachine.Name {
		t.Errorf("Name: got %q, want %q", state.Name.ValueString(), apiMachine.Name)
	}
}

func TestMachineResource_Create_APIError(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	configureResource(t, r, client)

	plan := buildPlan(t, schm, "pve1", "proxmox", "Dell", "R640")
	resp := &resource.CreateResponse{State: emptyState(schm)}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Create: expected error on API failure, got none")
	}
}

// --- Read ---

func TestMachineResource_Read_Found(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	apiMachine := apiclient.Machine{
		ID:    "uuid-read-1",
		Name:  "nas01",
		Kind:  "nas",
		Make:  "Synology",
		Model: "DS920+",
		Notes: "main storage",
	}
	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		writeMachine(w, http.StatusOK, apiMachine)
	})
	configureResource(t, r, client)

	initialState := buildState(t, schm, apiclient.Machine{ID: "uuid-read-1"})
	resp := &resource.ReadResponse{State: initialState}
	r.Read(ctx, resource.ReadRequest{State: initialState}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: unexpected error: %v", resp.Diagnostics)
	}

	var state testMachineModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("Read: state.Get: %v", diags)
	}
	if state.Name.ValueString() != apiMachine.Name {
		t.Errorf("Name: got %q, want %q", state.Name.ValueString(), apiMachine.Name)
	}
	if state.Notes.ValueString() != apiMachine.Notes {
		t.Errorf("Notes: got %q, want %q", state.Notes.ValueString(), apiMachine.Notes)
	}
}

func TestMachineResource_Read_NotFound_RemovesResource(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	configureResource(t, r, client)

	initialState := buildState(t, schm, apiclient.Machine{ID: "gone-id"})
	resp := &resource.ReadResponse{State: initialState}
	r.Read(ctx, resource.ReadRequest{State: initialState}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read(not found): unexpected error: %v", resp.Diagnostics)
	}
	// When the resource no longer exists, the framework marks state as removed
	// (null). The State.Raw should be null.
	if !resp.State.Raw.IsNull() {
		t.Error("Read(not found): state should be null (resource removed)")
	}
}

func TestMachineResource_Read_APIError(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	configureResource(t, r, client)

	initialState := buildState(t, schm, apiclient.Machine{ID: "some-id"})
	resp := &resource.ReadResponse{State: initialState}
	r.Read(ctx, resource.ReadRequest{State: initialState}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Read: expected error on API failure, got none")
	}
}

// --- Update ---

func TestMachineResource_Update_Success(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	original := apiclient.Machine{
		ID: "uuid-update-1", Name: "nas01", Kind: "nas", Make: "Synology", Model: "DS920+",
	}
	updated := apiclient.Machine{
		ID: "uuid-update-1", Name: "nas01", Kind: "nas", Make: "Synology", Model: "DS923+", RAMGB: 8,
	}
	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Errorf("Update: method: got %q, want PUT", req.Method)
		}
		writeMachine(w, http.StatusOK, updated)
	})
	configureResource(t, r, client)

	plan := buildPlan(t, schm, "nas01", "nas", "Synology", "DS923+")
	currentState := buildState(t, schm, original)
	resp := &resource.UpdateResponse{State: currentState}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: currentState}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: unexpected error: %v", resp.Diagnostics)
	}

	var state testMachineModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("Update: state.Get: %v", diags)
	}
	if state.Model.ValueString() != updated.Model {
		t.Errorf("Model: got %q, want %q", state.Model.ValueString(), updated.Model)
	}
	if state.RAMGB.ValueInt64() != updated.RAMGB {
		t.Errorf("RAMGB: got %d, want %d", state.RAMGB.ValueInt64(), updated.RAMGB)
	}
	// ID must be preserved from original state.
	if state.ID.ValueString() != original.ID {
		t.Errorf("ID: got %q, want %q (ID should be preserved)", state.ID.ValueString(), original.ID)
	}
}

func TestMachineResource_Update_APIError(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	configureResource(t, r, client)

	plan := buildPlan(t, schm, "nas01", "nas", "Synology", "DS923+")
	currentState := buildState(t, schm, apiclient.Machine{ID: "ghost-id"})
	resp := &resource.UpdateResponse{State: currentState}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: currentState}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Update: expected error on API failure, got none")
	}
}

// --- Delete ---

func TestMachineResource_Delete_Success(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Errorf("Delete: method: got %q, want DELETE", req.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	configureResource(t, r, client)

	currentState := buildState(t, schm, apiclient.Machine{ID: "uuid-delete-1"})
	resp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: currentState}, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("Delete: unexpected error: %v", resp.Diagnostics)
	}
}

func TestMachineResource_Delete_APIError(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	configureResource(t, r, client)

	currentState := buildState(t, schm, apiclient.Machine{ID: "uuid-delete-err"})
	resp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: currentState}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Delete: expected error on API failure, got none")
	}
}

// --- ImportState ---

func TestMachineResource_ImportState_Found(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	ri, ok := r.(resource.ResourceWithImportState)
	if !ok {
		t.Fatal("resource does not implement ResourceWithImportState")
	}

	apiMachine := apiclient.Machine{
		ID:   "uuid-import-1",
		Name: "pve2",
		Kind: "proxmox",
		Make: "HP",
		Model: "DL380",
	}
	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		writeMachine(w, http.StatusOK, apiMachine)
	})
	configureResource(t, r, client)

	resp := &resource.ImportStateResponse{State: emptyState(schm)}
	ri.ImportState(ctx, resource.ImportStateRequest{ID: "uuid-import-1"}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState: unexpected error: %v", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("ImportState: state should not be null")
	}

	var state testMachineModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatalf("ImportState: state.Get: %v", diags)
	}
	if state.ID.ValueString() != apiMachine.ID {
		t.Errorf("ID: got %q, want %q", state.ID.ValueString(), apiMachine.ID)
	}
	if state.Name.ValueString() != apiMachine.Name {
		t.Errorf("Name: got %q, want %q", state.Name.ValueString(), apiMachine.Name)
	}
}

func TestMachineResource_ImportState_NotFound(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	ri := r.(resource.ResourceWithImportState)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	configureResource(t, r, client)

	resp := &resource.ImportStateResponse{State: emptyState(schm)}
	ri.ImportState(ctx, resource.ImportStateRequest{ID: "missing-id"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("ImportState(not found): expected error, got none")
	}
}

func TestMachineResource_ImportState_APIError(t *testing.T) {
	ctx := context.Background()
	r := resources.NewMachineResource()
	schm := getSchema(t, r)

	ri := r.(resource.ResourceWithImportState)

	client := newMockServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	configureResource(t, r, client)

	resp := &resource.ImportStateResponse{State: emptyState(schm)}
	ri.ImportState(ctx, resource.ImportStateRequest{ID: "some-id"}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("ImportState(API error): expected error, got none")
	}
}
