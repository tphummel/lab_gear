package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tomflanagan/terraform-provider-lab/internal/labapi"
)

var _ resource.Resource = &MachineResource{}
var _ resource.ResourceWithImportState = &MachineResource{}

type MachineResource struct {
	client *labapi.Client
}

type MachineResourceModel struct {
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
	CreatedAt types.String  `tfsdk:"created_at"`
	UpdatedAt types.String  `tfsdk:"updated_at"`
}

func NewMachineResource() resource.Resource {
	return &MachineResource{}
}

func (r *MachineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

func (r *MachineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Represents a physical machine inventory record in lab-assets.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-generated UUID for the machine.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{Required: true},
			"kind": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("proxmox", "nas", "sbc", "bare_metal", "workstation", "laptop"),
				},
			},
			"make":       schema.StringAttribute{Required: true},
			"model":      schema.StringAttribute{Required: true},
			"cpu":        schema.StringAttribute{Optional: true},
			"ram_gb":     schema.Int64Attribute{Optional: true},
			"storage_tb": schema.Float64Attribute{Optional: true},
			"location":   schema.StringAttribute{Optional: true},
			"serial":     schema.StringAttribute{Optional: true},
			"notes":      schema.StringAttribute{Optional: true},
			"created_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *MachineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*labapi.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *labapi.Client, got: %T", req.ProviderData))
		return
	}

	r.client = client
}

func (r *MachineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The lab provider client is not configured.")
		return
	}

	var plan MachineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateMachine(ctx, machineFromModel(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating machine", err.Error())
		return
	}

	state := modelFromMachine(*created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MachineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The lab provider client is not configured.")
		return
	}

	var state MachineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	machine, err := r.client.GetMachine(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr labapi.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading machine", err.Error())
		return
	}

	newState := modelFromMachine(*machine)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *MachineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The lab provider client is not configured.")
		return
	}

	var plan MachineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateMachine(ctx, plan.ID.ValueString(), machineFromModel(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating machine", err.Error())
		return
	}

	newState := modelFromMachine(*updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *MachineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The lab provider client is not configured.")
		return
	}

	var state MachineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteMachine(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr labapi.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error deleting machine", err.Error())
	}
}

func (r *MachineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func machineFromModel(model MachineResourceModel) labapi.Machine {
	machine := labapi.Machine{
		Name:  model.Name.ValueString(),
		Kind:  model.Kind.ValueString(),
		Make:  model.Make.ValueString(),
		Model: model.Model.ValueString(),
	}

	if !model.CPU.IsNull() && !model.CPU.IsUnknown() {
		cpu := model.CPU.ValueString()
		machine.CPU = &cpu
	}
	if !model.RAMGB.IsNull() && !model.RAMGB.IsUnknown() {
		ram := model.RAMGB.ValueInt64()
		machine.RAMGB = &ram
	}
	if !model.StorageTB.IsNull() && !model.StorageTB.IsUnknown() {
		storage := model.StorageTB.ValueFloat64()
		machine.StorageTB = &storage
	}
	if !model.Location.IsNull() && !model.Location.IsUnknown() {
		location := model.Location.ValueString()
		machine.Location = &location
	}
	if !model.Serial.IsNull() && !model.Serial.IsUnknown() {
		serial := model.Serial.ValueString()
		machine.Serial = &serial
	}
	if !model.Notes.IsNull() && !model.Notes.IsUnknown() {
		notes := model.Notes.ValueString()
		machine.Notes = &notes
	}

	return machine
}

func modelFromMachine(machine labapi.Machine) MachineResourceModel {
	state := MachineResourceModel{
		ID:        types.StringValue(machine.ID),
		Name:      types.StringValue(machine.Name),
		Kind:      types.StringValue(machine.Kind),
		Make:      types.StringValue(machine.Make),
		Model:     types.StringValue(machine.Model),
		CreatedAt: types.StringValue(machine.CreatedAt),
		UpdatedAt: types.StringValue(machine.UpdatedAt),
		CPU:       types.StringNull(),
		RAMGB:     types.Int64Null(),
		StorageTB: types.Float64Null(),
		Location:  types.StringNull(),
		Serial:    types.StringNull(),
		Notes:     types.StringNull(),
	}

	if machine.CPU != nil {
		state.CPU = types.StringValue(*machine.CPU)
	}
	if machine.RAMGB != nil {
		state.RAMGB = types.Int64Value(*machine.RAMGB)
	}
	if machine.StorageTB != nil {
		state.StorageTB = types.Float64Value(*machine.StorageTB)
	}
	if machine.Location != nil {
		state.Location = types.StringValue(*machine.Location)
	}
	if machine.Serial != nil {
		state.Serial = types.StringValue(*machine.Serial)
	}
	if machine.Notes != nil {
		state.Notes = types.StringValue(*machine.Notes)
	}

	return state
}
