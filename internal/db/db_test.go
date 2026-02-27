package db_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/tphummel/lab_gear/internal/db"
	"github.com/tphummel/lab_gear/internal/models"
)

// newTestDB opens a fresh in-memory SQLite database for each test.
func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// sampleMachine returns a fully-populated Machine for use in tests.
func sampleMachine(id string) *models.Machine {
	now := time.Now().UTC().Truncate(time.Second)
	return &models.Machine{
		ID:        id,
		Name:      "pve2",
		Kind:      "proxmox",
		Make:      "Dell",
		Model:     "OptiPlex 7050",
		CPU:       "i7-7700",
		RAMGB:     32,
		StorageTB: 1.0,
		Location:  "office rack",
		Serial:    "SN-001",
		Notes:     "primary hypervisor",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNew(t *testing.T) {
	// Verifies schema is created and the DB is usable.
	d := newTestDB(t)
	if d == nil {
		t.Fatal("expected non-nil DB")
	}
}

func TestCreate_GetByID(t *testing.T) {
	d := newTestDB(t)
	m := sampleMachine("abc-123")

	if err := d.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := d.GetByID("abc-123")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID: got %q, want %q", got.ID, m.ID)
	}
	if got.Name != m.Name {
		t.Errorf("Name: got %q, want %q", got.Name, m.Name)
	}
	if got.Kind != m.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, m.Kind)
	}
	if got.Make != m.Make {
		t.Errorf("Make: got %q, want %q", got.Make, m.Make)
	}
	if got.Model != m.Model {
		t.Errorf("Model: got %q, want %q", got.Model, m.Model)
	}
	if got.CPU != m.CPU {
		t.Errorf("CPU: got %q, want %q", got.CPU, m.CPU)
	}
	if got.RAMGB != m.RAMGB {
		t.Errorf("RAMGB: got %d, want %d", got.RAMGB, m.RAMGB)
	}
	if got.StorageTB != m.StorageTB {
		t.Errorf("StorageTB: got %f, want %f", got.StorageTB, m.StorageTB)
	}
	if got.Location != m.Location {
		t.Errorf("Location: got %q, want %q", got.Location, m.Location)
	}
	if got.Serial != m.Serial {
		t.Errorf("Serial: got %q, want %q", got.Serial, m.Serial)
	}
	if got.Notes != m.Notes {
		t.Errorf("Notes: got %q, want %q", got.Notes, m.Notes)
	}
	if !got.CreatedAt.Equal(m.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, m.CreatedAt)
	}
	if !got.UpdatedAt.Equal(m.UpdatedAt) {
		t.Errorf("UpdatedAt: got %v, want %v", got.UpdatedAt, m.UpdatedAt)
	}
}

func TestCreate_OptionalFieldsDefault(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	m := &models.Machine{
		ID:        "min-001",
		Name:      "nas01",
		Kind:      "nas",
		Make:      "Synology",
		Model:     "DS920+",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := d.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := d.GetByID("min-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.CPU != "" {
		t.Errorf("CPU default: got %q, want empty", got.CPU)
	}
	if got.RAMGB != 0 {
		t.Errorf("RAMGB default: got %d, want 0", got.RAMGB)
	}
	if got.StorageTB != 0 {
		t.Errorf("StorageTB default: got %f, want 0", got.StorageTB)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetByID("does-not-exist")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestList_Empty(t *testing.T) {
	d := newTestDB(t)
	machines, err := d.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(machines) != 0 {
		t.Errorf("expected empty list, got %d items", len(machines))
	}
}

func TestList_All(t *testing.T) {
	d := newTestDB(t)

	m1 := sampleMachine("id-1")
	m1.Name = "pve1"
	m1.Kind = "proxmox"

	m2 := sampleMachine("id-2")
	m2.Name = "nas01"
	m2.Kind = "nas"

	for _, m := range []*models.Machine{m1, m2} {
		if err := d.Create(m); err != nil {
			t.Fatalf("Create %q: %v", m.ID, err)
		}
	}

	machines, err := d.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(machines) != 2 {
		t.Errorf("expected 2 machines, got %d", len(machines))
	}
}

func TestList_KindFilter(t *testing.T) {
	d := newTestDB(t)

	kinds := []struct {
		id   string
		kind string
	}{
		{"id-1", "proxmox"},
		{"id-2", "proxmox"},
		{"id-3", "nas"},
		{"id-4", "sbc"},
	}
	for _, k := range kinds {
		m := sampleMachine(k.id)
		m.Kind = k.kind
		if err := d.Create(m); err != nil {
			t.Fatalf("Create %q: %v", k.id, err)
		}
	}

	tests := []struct {
		kind string
		want int
	}{
		{"proxmox", 2},
		{"nas", 1},
		{"sbc", 1},
		{"laptop", 0},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got, err := d.List(tt.kind)
			if err != nil {
				t.Fatalf("List(%q): %v", tt.kind, err)
			}
			if len(got) != tt.want {
				t.Errorf("List(%q): got %d, want %d", tt.kind, len(got), tt.want)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	d := newTestDB(t)
	m := sampleMachine("upd-1")
	if err := d.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	m.Name = "pve2-renamed"
	m.RAMGB = 64
	m.StorageTB = 4.0
	m.Notes = "upgraded"
	m.UpdatedAt = time.Now().UTC().Truncate(time.Second).Add(time.Minute)

	if err := d.Update(m); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := d.GetByID("upd-1")
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Name != "pve2-renamed" {
		t.Errorf("Name: got %q, want %q", got.Name, "pve2-renamed")
	}
	if got.RAMGB != 64 {
		t.Errorf("RAMGB: got %d, want 64", got.RAMGB)
	}
	if got.StorageTB != 4.0 {
		t.Errorf("StorageTB: got %f, want 4.0", got.StorageTB)
	}
	if got.Notes != "upgraded" {
		t.Errorf("Notes: got %q, want %q", got.Notes, "upgraded")
	}
	// CreatedAt must not change
	if !got.CreatedAt.Equal(m.CreatedAt) {
		t.Errorf("CreatedAt changed: got %v, want %v", got.CreatedAt, m.CreatedAt)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	d := newTestDB(t)
	m := sampleMachine("ghost")
	err := d.Update(m)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	d := newTestDB(t)
	m := sampleMachine("del-1")
	if err := d.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := d.Delete("del-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := d.GetByID("del-1")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	d := newTestDB(t)
	err := d.Delete("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestCreate_DuplicateID(t *testing.T) {
	d := newTestDB(t)
	m := sampleMachine("dup-1")
	if err := d.Create(m); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := d.Create(m); err == nil {
		t.Error("expected error on duplicate ID, got nil")
	}
}
