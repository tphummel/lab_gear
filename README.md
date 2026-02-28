# lab_gear

A lightweight REST API for tracking physical machines in a homelab, paired with a custom Terraform provider for managing machine inventory as infrastructure-as-code.

## What it is

**lab_gear** (service: `lab-assets`) keeps a simple inventory of physical machines — hypervisors, NAS hosts, SBCs, bare metal servers, workstations, and laptops. It exposes them via a REST API backed by SQLite.

The companion **`terraform-provider-lab_gear`** lets you declare machines in Terraform HCL, review changes through pull requests via Atlantis, and reference physical hosts as explicit dependencies of logical infrastructure (e.g. an LXC container referencing the Proxmox node it runs on).

## Service

### Running locally

```bash
export API_TOKEN=secret
go run ./cmd/server
```

The service listens on port `8080` by default.

### Environment variables

| Variable    | Required | Default           | Description               |
|-------------|----------|-------------------|---------------------------|
| `API_TOKEN` | Yes      | —                 | Bearer token for API auth |
| `DB_PATH`   | No       | `./lab-assets.db` | Path to SQLite database   |
| `PORT`      | No       | `8080`            | Listen port               |

Use `DB_PATH=:memory:` for an ephemeral in-memory database (useful for testing).

### Building

```bash
make build          # produces bin/lab_gear
make docker-build   # builds Docker image tagged lab_gear
```

### Testing

```bash
make test           # run unit tests
make cover          # run tests with HTML coverage report
make smoke-test     # build + run k6 smoke tests against a live server
```

## API

Base URL: `http://localhost:8080`

All endpoints except `/healthz` require:

```
Authorization: Bearer <API_TOKEN>
```

### Endpoints

| Method   | Path                    | Description            |
|----------|-------------------------|------------------------|
| `GET`    | `/healthz`              | Health check (no auth) |
| `POST`   | `/api/v1/machines`      | Create a machine       |
| `GET`    | `/api/v1/machines`      | List all machines      |
| `GET`    | `/api/v1/machines/{id}` | Get a machine by ID    |
| `PUT`    | `/api/v1/machines/{id}` | Update a machine       |
| `DELETE` | `/api/v1/machines/{id}` | Delete a machine       |

Filter by kind: `GET /api/v1/machines?kind=proxmox`

### Create a machine

```bash
curl -s -X POST http://localhost:8080/api/v1/machines \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "pve2",
    "kind": "proxmox",
    "make": "Dell",
    "model": "OptiPlex 7050",
    "cpu": "i7-7700",
    "ram_gb": 32,
    "storage_tb": 1.0,
    "location": "office rack"
  }'
```

### List machines

```bash
curl -s http://localhost:8080/api/v1/machines \
  -H "Authorization: Bearer $API_TOKEN"
```

### Machine kinds

| Kind          | Description                                  |
|---------------|----------------------------------------------|
| `proxmox`     | Proxmox VE hypervisor node                   |
| `nas`         | Network-attached storage (Synology, TrueNAS) |
| `sbc`         | Single-board computer (Raspberry Pi, etc.)   |
| `bare_metal`  | Bare metal server, not running a hypervisor  |
| `workstation` | Desktop workstation                          |
| `laptop`       | Laptop                                       |

## Terraform Provider

The provider lives in `terraform-provider-lab_gear/`.

### Configuration

```hcl
terraform {
  required_providers {
    lab = {
      source = "registry.terraform.io/tomflanagan/lab"
    }
  }
}

provider "lab" {
  endpoint = "https://assets.lab.local"
  # api_key can also be set via LAB_API_KEY env var
}
```

| Config     | Env Var        | Description  |
|------------|----------------|--------------|
| `endpoint` | `LAB_ENDPOINT` | API base URL |
| `api_key`  | `LAB_API_KEY`  | Bearer token |

### Declaring machines

```hcl
resource "lab_machine" "pve2" {
  name       = "pve2"
  kind       = "proxmox"
  make       = "Dell"
  model      = "OptiPlex 7050"
  cpu        = "i7-7700"
  ram_gb     = 32
  storage_tb = 1.0
  location   = "office rack"
}

resource "lab_machine" "nas01" {
  name  = "nas01"
  kind  = "nas"
  make  = "Synology"
  model = "DS920+"
}

resource "lab_machine" "pi01" {
  name   = "pi01"
  kind   = "sbc"
  make   = "Raspberry Pi"
  model  = "4 Model B"
  ram_gb = 8
}
```

### Referencing machines from other resources

```hcl
resource "proxmox_lxc" "gitea" {
  target_node = lab_machine.pve2.name
  hostname    = "gitea"
  # ...
}
```

This makes the LXC container's Terraform plan dependent on the physical host record. If the host is renamed or removed from inventory, `terraform plan` will surface it as a change.

### Importing existing machines

```bash
terraform import lab_machine.pve2 <uuid>
```

The UUID is the `id` returned by the API when the machine was created.

### Building the provider

```bash
cd terraform-provider-lab_gear
make build
```

## Deployment

The service is designed to run in a dedicated LXC container behind a Caddy reverse proxy. See [DESIGN.md](DESIGN.md) for the full architecture, database schema, and deployment details.
