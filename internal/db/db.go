package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/tphummel/lab_gear/internal/models"
	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection.
type DB struct {
	conn *sql.DB
}

// New opens the SQLite database at path, enables WAL mode, and runs migrations.
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	if err := migrate(conn); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{conn: conn}, nil
}

func migrate(conn *sql.DB) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS machines (
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
		CREATE INDEX IF NOT EXISTS idx_machines_kind ON machines(kind);
		CREATE INDEX IF NOT EXISTS idx_machines_name ON machines(name);
	`)
	return err
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Ping verifies the database connection is alive.
func (d *DB) Ping() error {
	return d.conn.Ping()
}

// Create inserts a new machine record.
func (d *DB) Create(m *models.Machine) error {
	_, err := d.conn.Exec(`
		INSERT INTO machines (id, name, kind, make, model, cpu, ram_gb, storage_tb, location, serial, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.Kind, m.Make, m.Model, m.CPU, m.RAMGB, m.StorageTB,
		m.Location, m.Serial, m.Notes,
		m.CreatedAt.UTC().Format(time.RFC3339),
		m.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetByID returns the machine with the given ID, or sql.ErrNoRows if not found.
func (d *DB) GetByID(id string) (*models.Machine, error) {
	row := d.conn.QueryRow(`
		SELECT id, name, kind, make, model, cpu, ram_gb, storage_tb, location, serial, notes, created_at, updated_at
		FROM machines WHERE id = ?`, id)
	return scanRow(row)
}

// List returns all machines, optionally filtered by kind.
func (d *DB) List(kind string) ([]*models.Machine, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if kind != "" {
		rows, err = d.conn.Query(`
			SELECT id, name, kind, make, model, cpu, ram_gb, storage_tb, location, serial, notes, created_at, updated_at
			FROM machines WHERE kind = ?`, kind)
	} else {
		rows, err = d.conn.Query(`
			SELECT id, name, kind, make, model, cpu, ram_gb, storage_tb, location, serial, notes, created_at, updated_at
			FROM machines`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var machines []*models.Machine
	for rows.Next() {
		m, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		machines = append(machines, m)
	}
	return machines, rows.Err()
}

// Update replaces all mutable fields for the machine with m.ID.
// Returns sql.ErrNoRows if no such machine exists.
func (d *DB) Update(m *models.Machine) error {
	res, err := d.conn.Exec(`
		UPDATE machines
		SET name=?, kind=?, make=?, model=?, cpu=?, ram_gb=?, storage_tb=?, location=?, serial=?, notes=?, updated_at=?
		WHERE id=?`,
		m.Name, m.Kind, m.Make, m.Model, m.CPU, m.RAMGB, m.StorageTB,
		m.Location, m.Serial, m.Notes,
		m.UpdatedAt.UTC().Format(time.RFC3339),
		m.ID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes the machine with the given ID.
// Returns sql.ErrNoRows if no such machine exists.
func (d *DB) Delete(id string) error {
	res, err := d.conn.Exec(`DELETE FROM machines WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanRow(row *sql.Row) (*models.Machine, error) {
	var m models.Machine
	var createdAt, updatedAt string
	if err := row.Scan(
		&m.ID, &m.Name, &m.Kind, &m.Make, &m.Model,
		&m.CPU, &m.RAMGB, &m.StorageTB,
		&m.Location, &m.Serial, &m.Notes,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	var err error
	m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	return &m, nil
}

func scanRows(rows *sql.Rows) (*models.Machine, error) {
	var m models.Machine
	var createdAt, updatedAt string
	if err := rows.Scan(
		&m.ID, &m.Name, &m.Kind, &m.Make, &m.Model,
		&m.CPU, &m.RAMGB, &m.StorageTB,
		&m.Location, &m.Serial, &m.Notes,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	var err error
	m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	return &m, nil
}
