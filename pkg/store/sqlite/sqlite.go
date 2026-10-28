package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS devices (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	room TEXT NOT NULL DEFAULT '',
	capabilities TEXT NOT NULL DEFAULT '{}',
	shadow TEXT NOT NULL DEFAULT '{}',
	status TEXT NOT NULL DEFAULT 'offline',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	token_hint TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_devices_room ON devices(room);
CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(type);
CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
`

// Device row aligned with SPEC §5.1.
type Device struct {
	ID           int64
	DeviceID     string
	Type         string
	Name         string
	Room         string
	Capabilities json.RawMessage
	Shadow       json.RawMessage
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store wraps SQLite access for hub metadata.
type Store struct {
	db *sql.DB
}

// Open ensures the data directory exists and applies schema.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir data dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

// EnsureDemoUser inserts a placeholder demo user if missing.
func (s *Store) EnsureDemoUser(username string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO users (username, token_hint) VALUES (?, ?)`,
		username, "demo-bearer",
	)
	return err
}

// InsertDevice registers a new device.
func (s *Store) InsertDevice(d *Device) error {
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	if len(d.Capabilities) == 0 {
		d.Capabilities = json.RawMessage(`{}`)
	}
	if len(d.Shadow) == 0 {
		d.Shadow = json.RawMessage(`{}`)
	}
	if d.Status == "" {
		d.Status = "offline"
	}

	res, err := s.db.Exec(
		`INSERT INTO devices (device_id, type, name, room, capabilities, shadow, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.DeviceID, d.Type, d.Name, d.Room,
		string(d.Capabilities), string(d.Shadow), d.Status,
		d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	d.ID = id
	return nil
}

// GetDeviceByID loads one device by business id.
func (s *Store) GetDeviceByID(deviceID string) (*Device, error) {
	row := s.db.QueryRow(
		`SELECT id, device_id, type, name, room, capabilities, shadow, status, created_at, updated_at
		 FROM devices WHERE device_id = ?`, deviceID,
	)
	return scanDevice(row)
}

func scanDevice(row *sql.Row) (*Device, error) {
	var d Device
	var cap, sh, ca, ua string
	if err := row.Scan(&d.ID, &d.DeviceID, &d.Type, &d.Name, &d.Room,
		&cap, &sh, &d.Status, &ca, &ua); err != nil {
		return nil, err
	}
	d.Capabilities = json.RawMessage(cap)
	d.Shadow = json.RawMessage(sh)
	var err error
	d.CreatedAt, err = time.Parse(time.RFC3339, ca)
	if err != nil {
		d.CreatedAt, _ = time.Parse(time.RFC3339Nano, ca)
	}
	d.UpdatedAt, err = time.Parse(time.RFC3339, ua)
	if err != nil {
		d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, ua)
	}
	return &d, nil
}

// ListDevicesFilter optional filters (empty string means no filter).
type ListDevicesFilter struct {
	Room   string
	Type   string
	Status string
}

// ListDevices returns devices matching filters.
func (s *Store) ListDevices(f ListDevicesFilter) ([]Device, error) {
	q := `SELECT id, device_id, type, name, room, capabilities, shadow, status, created_at, updated_at FROM devices WHERE 1=1`
	args := []any{}
	if f.Room != "" {
		q += ` AND room = ?`
		args = append(args, f.Room)
	}
	if f.Type != "" {
		q += ` AND type = ?`
		args = append(args, f.Type)
	}
	if f.Status != "" {
		q += ` AND status = ?`
		args = append(args, f.Status)
	}
	q += ` ORDER BY device_id`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Device
	for rows.Next() {
		var d Device
		var cap, sh, ca, ua string
		if err := rows.Scan(&d.ID, &d.DeviceID, &d.Type, &d.Name, &d.Room,
			&cap, &sh, &d.Status, &ca, &ua); err != nil {
			return nil, err
		}
		d.Capabilities = json.RawMessage(cap)
		d.Shadow = json.RawMessage(sh)
		d.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		d.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateShadow merges payload into stored shadow JSON and updates status.
func (s *Store) UpdateShadow(deviceID string, merge map[string]any, status string) error {
	d, err := s.GetDeviceByID(deviceID)
	if err != nil {
		return err
	}
	var shadow map[string]any
	if err := json.Unmarshal(d.Shadow, &shadow); err != nil || shadow == nil {
		shadow = map[string]any{}
	}
	for k, v := range merge {
		shadow[k] = v
	}
	shBytes, err := json.Marshal(shadow)
	if err != nil {
		return err
	}
	if status == "" {
		status = d.Status
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(
		`UPDATE devices SET shadow = ?, status = ?, updated_at = ? WHERE device_id = ?`,
		string(shBytes), status, now, deviceID,
	)
	return err
}

// SetStatus updates device status only.
func (s *Store) SetStatus(deviceID, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE devices SET status = ?, updated_at = ? WHERE device_id = ?`,
		status, now, deviceID,
	)
	return err
}

// SetUnbound marks device unbound (simplified unbind).
func (s *Store) SetUnbound(deviceID string) error {
	return s.SetStatus(deviceID, "unbound")
}
