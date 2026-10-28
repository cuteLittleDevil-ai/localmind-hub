package sqlite

import (
	"strings"
	"testing"
)

func TestSchemaCreatesTables(t *testing.T) {
	store, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var name string
	err = store.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='devices'`,
	).Scan(&name)
	if err != nil || name != "devices" {
		t.Fatalf("devices table: %v name=%q", err, name)
	}
	err = store.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='users'`,
	).Scan(&name)
	if err != nil || name != "users" {
		t.Fatalf("users table: %v name=%q", err, name)
	}
}

func TestInsertDeviceDuplicateConflict(t *testing.T) {
	store, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	d := &Device{DeviceID: "light-living-01", Type: "light", Name: "客厅灯", Room: "living"}
	if err := store.InsertDevice(d); err != nil {
		t.Fatal(err)
	}
	err = store.InsertDevice(&Device{DeviceID: "light-living-01", Type: "light", Name: "x", Room: "living"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") &&
		!strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Fatalf("unexpected err: %v", err)
	}
}
