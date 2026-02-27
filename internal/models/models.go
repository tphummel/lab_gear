package models

import "time"

// Machine represents a physical machine in the homelab inventory.
type Machine struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Make      string    `json:"make"`
	Model     string    `json:"model"`
	CPU       string    `json:"cpu"`
	RAMGB     int       `json:"ram_gb"`
	StorageTB float64   `json:"storage_tb"`
	Location  string    `json:"location"`
	Serial    string    `json:"serial"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ValidKinds is the set of allowed machine kind values.
var ValidKinds = map[string]bool{
	"proxmox":     true,
	"nas":         true,
	"sbc":         true,
	"bare_metal":  true,
	"workstation": true,
	"laptop":      true,
}
