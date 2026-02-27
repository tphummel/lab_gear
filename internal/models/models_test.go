package models_test

import (
	"testing"

	"github.com/tphummel/lab_gear/internal/models"
)

func TestValidKinds_ContainsExpectedValues(t *testing.T) {
	expected := []string{"proxmox", "nas", "sbc", "bare_metal", "workstation", "laptop"}

	if len(models.ValidKinds) != len(expected) {
		t.Errorf("ValidKinds: got %d entries, want %d", len(models.ValidKinds), len(expected))
	}

	for _, k := range expected {
		if !models.ValidKinds[k] {
			t.Errorf("ValidKinds: missing expected kind %q", k)
		}
	}
}

func TestValidKinds_RejectsInvalidKind(t *testing.T) {
	invalid := []string{"mainframe", "toaster", "server", "", "PROXMOX", "Nas"}
	for _, k := range invalid {
		if models.ValidKinds[k] {
			t.Errorf("ValidKinds: should not contain %q", k)
		}
	}
}

func TestValidKinds_IsCaseSensitive(t *testing.T) {
	if models.ValidKinds["Proxmox"] {
		t.Error("ValidKinds should be case-sensitive; 'Proxmox' should not match 'proxmox'")
	}
	if models.ValidKinds["NAS"] {
		t.Error("ValidKinds should be case-sensitive; 'NAS' should not match 'nas'")
	}
}
