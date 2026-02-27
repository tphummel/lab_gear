package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/provider"
)

type machineResource struct {
	client *provider.Client
}

// machineModel maps the Terraform schema attributes to Go values.
type machineModel struct {
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

// NewMachineResource is the factory function registered with the provider.
func NewMachineResource() resource.Resource {
	return &machineResource{}
}

func (r *machineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine" // → "lab_gear_machine"
}

func (r *machineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a physical machine in the lab_gear inventory.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Server-generated UUID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name":  schema.StringAttribute{Description: "Handle for the machine (e.g. pve2, nas01).", Required: true},
			"kind":  schema.StringAttribute{Description: "Machine type: proxmox, nas, sbc, bare_metal, workstation, laptop.", Required: true},
			"make":  schema.StringAttribute{Description: "Manufacturer.", Required: true},
			"model": schema.StringAttribute{Description: "Model name or number.", Required: true},
			"cpu": schema.StringAttribute{
				Description: "CPU model.",
				Optional:    true,
				Computed:    true,
			},
			"ram_gb": schema.Int64Attribute{
				Description: "RAM in gigabytes.",
				Optional:    true,
				Computed:    true,
			},
			"storage_tb": schema.Float64Attribute{
				Description: "Total storage in terabytes.",
				Optional:    true,
				Computed:    true,
			},
			"location": schema.StringAttribute{
				Description: "Physical location.",
				Optional:    true,
				Computed:    true,
			},
			"serial": schema.StringAttribute{
				Description: "Serial number.",
				Optional:    true,
				Computed:    true,
			},
			"notes": schema.StringAttribute{
				Description: "Free-form notes.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *machineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*provider.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *provider.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *machineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan machineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateMachine(ctx, provider.Machine{
		Name:      plan.Name.ValueString(),
		Kind:      plan.Kind.ValueString(),
		Make:      plan.Make.ValueString(),
		Model:     plan.Model.ValueString(),
		CPU:       plan.CPU.ValueString(),
		RAMGB:     plan.RAMGB.ValueInt64(),
		StorageTB: plan.StorageTB.ValueFloat64(),
		Location:  plan.Location.ValueString(),
		Serial:    plan.Serial.ValueString(),
		Notes:     plan.Notes.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating lab_gear_machine", err.Error())
		return
	}

	machineToState(created, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *machineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	m, err := r.client.GetMachine(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading lab_gear_machine", err.Error())
		return
	}
	if m == nil {
		// Removed outside Terraform — tell the framework the resource is gone.
		resp.State.RemoveResource(ctx)
		return
	}

	machineToState(m, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *machineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan machineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateMachine(ctx, provider.Machine{
		ID:        state.ID.ValueString(),
		Name:      plan.Name.ValueString(),
		Kind:      plan.Kind.ValueString(),
		Make:      plan.Make.ValueString(),
		Model:     plan.Model.ValueString(),
		CPU:       plan.CPU.ValueString(),
		RAMGB:     plan.RAMGB.ValueInt64(),
		StorageTB: plan.StorageTB.ValueFloat64(),
		Location:  plan.Location.ValueString(),
		Serial:    plan.Serial.ValueString(),
		Notes:     plan.Notes.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating lab_gear_machine", err.Error())
		return
	}

	plan.ID = state.ID // preserve server-assigned ID
	machineToState(updated, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *machineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMachine(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting lab_gear_machine", err.Error())
	}
}

// ImportState enables: terraform import lab_gear_machine.pve2 <uuid>
func (r *machineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	m, err := r.client.GetMachine(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing lab_gear_machine", err.Error())
		return
	}
	if m == nil {
		resp.Diagnostics.AddError("Machine not found",
			fmt.Sprintf("No machine with ID %q exists in the lab_gear service.", req.ID))
		return
	}

	var state machineModel
	state.ID = types.StringValue(m.ID)
	machineToState(m, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// machineToState copies API response fields into the Terraform state model.
func machineToState(m *provider.Machine, s *machineModel) {
	s.ID = types.StringValue(m.ID)
	s.Name = types.StringValue(m.Name)
	s.Kind = types.StringValue(m.Kind)
	s.Make = types.StringValue(m.Make)
	s.Model = types.StringValue(m.Model)
	s.CPU = types.StringValue(m.CPU)
	s.RAMGB = types.Int64Value(m.RAMGB)
	s.StorageTB = types.Float64Value(m.StorageTB)
	s.Location = types.StringValue(m.Location)
	s.Serial = types.StringValue(m.Serial)
	s.Notes = types.StringValue(m.Notes)
}
