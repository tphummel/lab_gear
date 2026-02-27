# lab-assets: Design Document

## Overview

**lab-assets** is a lightweight REST API service for tracking physical machines in a homelab environment. It is paired with a custom Terraform provider (`lab`) that enables machine inventory to be managed as infrastructure-as-code, integrated into an existing Gitea + Atlantis GitOps workflow.

The primary resource is `lab_machine` — a physical device such as a Proxmox hypervisor, NAS, Raspberry Pi, bare metal server, workstation, or laptop.

## Problem Statement

Physical machine inventory in the homelab is currently implicit. When Terraform provisions an LXC container on `pve2`, there is no formal record of what `pve2` actually is — its hardware specs, location, serial number, or even that it exists. As the homelab grows to include NAS hosts, SBCs, bare metal servers, and workstations, having a single source of truth for physical assets becomes increasingly valuable for capacity planning, dependency tracking, and network policy.

## Goals

- Provide a queryable inventory of all physical machines in the homelab.
- Expose machines as Terraform resources so physical inventory is version-controlled and reviewed via Atlantis pull requests.
- Allow LXC and other provisioning resources to reference machine records, creating an explicit dependency between logical infrastructure and physical hosts.
- Keep it simple: one resource type, one table, minimal operational overhead.

## Non-Goals

- Replacing UniFi, Proxmox, or any other control plane.
- Monitoring, alerting, or health checking.
- Managing network devices, UPS units, or other non-compute assets (these may be added later as separate resource types).
- Public registry publishing for the Terraform provider.

## Architecture

```
┌──────────────────────┐
│  Gitea Repository    │
│  (Terraform HCL)     │
└──────────┬───────────┘
           │ PR
           ▼
┌──────────────────────┐
│  Atlantis            │
│  (plan/apply)        │
└──────────┬───────────┘
           │ API calls
           ▼
┌──────────────────────┐     ┌─────────────┐
│  lab-assets          │────▶│  SQLite     │
│  (Go, port 8080)     │     │  (WAL mode) │
└──────────────────────┘     └─────────────┘
           │
     Bearer token auth
     Caddy reverse proxy
```

The service runs in a dedicated LXC container, fronted by Caddy with Cloudflare DNS challenge for TLS. Atlantis calls the API via the `lab` Terraform provider during plan and apply.

## Resource: `lab_machine`

### Fields

|Field       |Type    |Required|Mutable|Description                                      |
|------------|--------|--------|-------|-------------------------------------------------|
|`id`        |string  |—       |No     |Server-generated UUID. Primary key.              |
|`name`      |string  |Yes     |Yes    |Handle for this machine (e.g. `pve2`, `nas01`).  |
|`kind`      |string  |Yes     |Yes    |Machine type. See valid kinds below.             |
|`make`      |string  |Yes     |Yes    |Manufacturer (e.g. Dell, Synology, Raspberry Pi).|
|`model`     |string  |Yes     |Yes    |Model name or number.                            |
|`cpu`       |string  |No      |Yes    |CPU model.                                       |
|`ram_gb`    |integer |No      |Yes    |RAM in gigabytes.                                |
|`storage_tb`|float   |No      |Yes    |Total storage in terabytes.                      |
|`location`  |string  |No      |Yes    |Physical location (e.g. office rack, closet).    |
|`serial`    |string  |No      |Yes    |Serial number.                                   |
|`notes`     |string  |No      |Yes    |Free-form notes.                                 |
|`created_at`|datetime|—       |No     |Server-generated creation timestamp.             |
|`updated_at`|datetime|—       |No     |Server-generated last update timestamp.          |

### Valid Kinds

|Kind         |Description                                 |
|-------------|--------------------------------------------|
|`proxmox`    |Proxmox VE hypervisor node                  |
|`nas`        |Network-attached storage (Synology, TrueNAS)|
|`sbc`        |Single-board computer (Raspberry Pi, etc.)  |
|`bare_metal` |Bare metal server, not running a hypervisor |
|`workstation`|Desktop workstation                         |
|`laptop`     |Laptop                                      |

### Idempotency

The `id` field is a server-generated UUID assigned at creation time. Terraform stores this ID in state after the initial `POST`. Subsequent `terraform plan` runs read by ID and diff against desired state. This makes the resource naturally idempotent — Terraform knows whether to create, update, or no-op based on the ID in state.

Unlike a name-keyed idempotency model, this allows multiple machines to share a name (though that would be unusual) and avoids coupling the API’s identity model to any particular client convention.

## API Design

Base path: `/api/v1`

### Endpoints

|Method  |Path                   |Description           |Response   |
|--------|-----------------------|----------------------|-----------|
|`GET`   |`/healthz`             |Health check (no auth)|`200`      |
|`POST`  |`/api/v1/machines`     |Create a machine      |`201`      |
|`GET`   |`/api/v1/machines`     |List all machines     |`200`      |
|`GET`   |`/api/v1/machines/{id}`|Get a machine by ID   |`200`/`404`|
|`PUT`   |`/api/v1/machines/{id}`|Update a machine      |`200`/`404`|
|`DELETE`|`/api/v1/machines/{id}`|Delete a machine      |`204`/`404`|

### Query Parameters

`GET /api/v1/machines` supports an optional `?kind=` filter to list machines of a specific type.

### Authentication

All endpoints except `/healthz` require a `Authorization: Bearer <token>` header. The token is a static secret loaded from the `API_TOKEN` environment variable. This is sufficient for an internal service behind Caddy/Cloudflare with a single consumer (Atlantis).

### Request/Response Format

All request and response bodies are JSON. Timestamps are RFC 3339.

**Create request:**

```json
{
  "name": "pve2",
  "kind": "proxmox",
  "make": "Dell",
  "model": "OptiPlex 7050",
  "cpu": "i7-7700",
  "ram_gb": 32,
  "storage_tb": 1.0,
  "location": "office rack"
}
```

**Response:**

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "name": "pve2",
  "kind": "proxmox",
  "make": "Dell",
  "model": "OptiPlex 7050",
  "cpu": "i7-7700",
  "ram_gb": 32,
  "storage_tb": 1.0,
  "location": "office rack",
  "serial": "",
  "notes": "",
  "created_at": "2026-02-26T12:00:00Z",
  "updated_at": "2026-02-26T12:00:00Z"
}
```

### Error Format

```json
{
  "error": "name, kind, make, and model are required"
}
```

## Database

SQLite in WAL mode. Single table.

```sql
CREATE TABLE machines (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL,
    make       TEXT NOT NULL,
    model      TEXT NOT NULL,
    cpu        TEXT NOT NULL DEFAULT '',
    ram_gb     INTEGER NOT NULL DEFAULT 0,
    storage_tb REAL NOT NULL DEFAULT 0,
    location   TEXT NOT NULL DEFAULT '',
    serial     TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_machines_kind ON machines(kind);
CREATE INDEX idx_machines_name ON machines(name);
```

The pure-Go SQLite driver (`modernc.org/sqlite`) is used to avoid CGO and simplify cross-compilation and container builds.

## Terraform Provider

### Provider Configuration

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
  # api_key via LAB_API_KEY env var
}
```

|Config    |Env Var       |Description |
|----------|--------------|------------|
|`endpoint`|`LAB_ENDPOINT`|API base URL|
|`api_key` |`LAB_API_KEY` |Bearer token|

Environment variables take lowest precedence; explicit config overrides them.

### Resource: `lab_machine`

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

### CRUD Mapping

|Terraform Operation|HTTP Method|Path                   |
|-------------------|-----------|-----------------------|
|Create             |`POST`     |`/api/v1/machines`     |
|Read               |`GET`      |`/api/v1/machines/{id}`|
|Update             |`PUT`      |`/api/v1/machines/{id}`|
|Delete             |`DELETE`   |`/api/v1/machines/{id}`|

### Import

Existing machines can be imported by their server-generated ID:

```bash
terraform import lab_machine.pve2 f47ac10b-58cc-4372-a567-0e02b2c3d479
```

### Integration with LXC Provisioning

The primary integration point is referencing `lab_machine` names as Proxmox target nodes:

```hcl
resource "lab_machine" "pve2" {
  name = "pve2"
  kind = "proxmox"
  make = "Dell"
  model = "OptiPlex 7050"
}

resource "proxmox_lxc" "gitea" {
  target_node = lab_machine.pve2.name
  hostname    = "gitea"
  # ...
}
```

This creates a Terraform dependency: the LXC container explicitly depends on the physical host existing in inventory. Deleting or renaming a host in the inventory will surface in `terraform plan` as a change to all containers running on it.

## Service Implementation

### Technology Choices

|Component      |Choice                      |Rationale                                                  |
|---------------|----------------------------|-----------------------------------------------------------|
|Language       |Go 1.22                     |Single binary, easy cross-compilation, good stdlib HTTP    |
|Database       |SQLite (WAL mode)           |Zero-ops, file-based, sufficient for single-writer workload|
|SQLite driver  |`modernc.org/sqlite`        |Pure Go, no CGO required                                   |
|HTTP router    |`net/http.ServeMux`         |Standard library, no dependencies                          |
|UUID generation|`github.com/google/uuid`    |Well-tested, v4 UUIDs                                      |
|Container      |Alpine-based multi-stage    |Minimal image size, no CGO means static binary works       |
|Provider SDK   |`terraform-plugin-framework`|HashiCorp’s current recommended SDK for new providers      |

### Project Structure

```
lab-assets/
├── cmd/server/main.go          # Entrypoint
├── internal/
│   ├── db/db.go                # SQLite operations
│   ├── handlers/handlers.go    # HTTP handlers
│   ├── middleware/auth.go      # Bearer token auth
│   └── models/models.go       # Data types
├── Dockerfile
├── Makefile
└── go.mod

terraform-provider-lab/
├── main.go                     # Provider server entrypoint
├── internal/
│   ├── provider/
│   │   ├── provider.go         # Provider config and schema
│   │   └── client.go           # HTTP client for lab-assets API
│   └── resources/
│       └── machine.go          # lab_machine resource CRUD
├── Makefile
└── go.mod
```

### Environment Variables

**Service (lab-assets):**

|Variable   |Required|Default          |Description              |
|-----------|--------|-----------------|-------------------------|
|`API_TOKEN`|Yes     |—                |Bearer token for API auth|
|`DB_PATH`  |No      |`./lab-assets.db`|Path to SQLite database  |
|`PORT`     |No      |`8080`           |Listen port              |

**Provider (terraform-provider-lab):**

|Variable      |Required|Description                 |
|--------------|--------|----------------------------|
|`LAB_ENDPOINT`|Yes*    |API base URL (or set in HCL)|
|`LAB_API_KEY` |Yes*    |Bearer token (or set in HCL)|

## Deployment

### LXC Container

The service runs in a dedicated LXC container provisioned via Terraform. The SQLite database file should be stored on a persistent volume or bind mount to survive container recreation.

### Reverse Proxy

Caddy handles TLS termination with a Cloudflare DNS challenge, consistent with other homelab services. Example Caddyfile snippet:

```
assets.lab.local {
    reverse_proxy localhost:8080
    tls {
        dns cloudflare {env.CF_API_TOKEN}
    }
}
```

### Atlantis Integration

The `LAB_ENDPOINT` and `LAB_API_KEY` environment variables are set in the Atlantis server environment. No special Atlantis configuration is required — the provider works like any other Terraform provider.

## Future Considerations

- **Additional resource types**: `lab_switch`, `lab_ups`, `lab_accesspoint` could be added as the inventory grows. Each would be a separate table and Terraform resource.
- **Data sources**: A `data.lab_machines` data source for querying/filtering machines without managing them (useful for read-only references in other modules).
- **Structured logging**: Add `slog` middleware for request logging before production use.
- **Backup**: Periodic SQLite backup via Litestream or a simple cron job copying the database file.
- **Migration framework**: If the schema evolves, a proper migration system (goose, golang-migrate) would replace the current `CREATE TABLE IF NOT EXISTS` approach.
## Provider Build & Publish Automation

The repository includes a GitHub Actions workflow at `.github/workflows/provider-release.yml` that builds the Terraform provider on every commit to `main`.

- **On push to `main`**: runs tests, cross-compiles provider binaries, and uploads them as GitHub Actions artifacts.
- **On tag `v*`**: additionally publishes zipped binaries and checksums to a GitHub Release.

This provides repeatable artifacts from every main-branch commit while keeping versioned release assets for downstream consumption.
