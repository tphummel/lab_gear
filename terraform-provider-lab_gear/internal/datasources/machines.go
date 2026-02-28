package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/apiclient"
)

// Ensure full interface compliance at compile time.
var _ datasource.DataSource = &machinesDataSource{}
var _ datasource.DataSourceWithConfigure = &machinesDataSource{}

type machinesDataSource struct {
	client *apiclient.Client
}

// NewMachinesDataSource is the factory function registered with the provider.
func NewMachinesDataSource() datasource.DataSource {
	return &machinesDataSource{}
}

type machinesDataSourceModel struct {
	Kind     types.String       `tfsdk:"kind"`
	Machines []machineDataModel `tfsdk:"machines"`
}

type machineDataModel struct {
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

func (d *machinesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machines" // → "lab_gear_machines"
}

func (d *machinesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists machines from the lab_gear inventory, optionally filtered by kind.",
		Attributes: map[string]schema.Attribute{
			"kind": schema.StringAttribute{
				Description: "Optional machine type filter (proxmox, nas, sbc, bare_metal, workstation, laptop).",
				Optional:    true,
			},
			"machines": schema.ListNestedAttribute{
				Description: "List of machines returned by the API.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true, Description: "Server-generated UUID."},
						"name":       schema.StringAttribute{Computed: true, Description: "Handle for the machine."},
						"kind":       schema.StringAttribute{Computed: true, Description: "Machine type."},
						"make":       schema.StringAttribute{Computed: true, Description: "Manufacturer."},
						"model":      schema.StringAttribute{Computed: true, Description: "Model name or number."},
						"cpu":        schema.StringAttribute{Computed: true, Description: "CPU model."},
						"ram_gb":     schema.Int64Attribute{Computed: true, Description: "RAM in gigabytes."},
						"storage_tb": schema.Float64Attribute{Computed: true, Description: "Total storage in terabytes."},
						"location":   schema.StringAttribute{Computed: true, Description: "Physical location."},
						"serial":     schema.StringAttribute{Computed: true, Description: "Serial number."},
						"notes":      schema.StringAttribute{Computed: true, Description: "Free-form notes."},
					},
				},
			},
		},
	}
}

func (d *machinesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *apiclient.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *machinesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state machinesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	machines, err := d.client.ListMachines(ctx, state.Kind.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing lab_gear machines", err.Error())
		return
	}

	state.Machines = make([]machineDataModel, len(machines))
	for i, m := range machines {
		state.Machines[i] = machineDataModel{
			ID:        types.StringValue(m.ID),
			Name:      types.StringValue(m.Name),
			Kind:      types.StringValue(m.Kind),
			Make:      types.StringValue(m.Make),
			Model:     types.StringValue(m.Model),
			CPU:       types.StringValue(m.CPU),
			RAMGB:     types.Int64Value(m.RAMGB),
			StorageTB: types.Float64Value(m.StorageTB),
			Location:  types.StringValue(m.Location),
			Serial:    types.StringValue(m.Serial),
			Notes:     types.StringValue(m.Notes),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
