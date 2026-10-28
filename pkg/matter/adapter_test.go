package matter

import "testing"

func TestNoopAdapter(t *testing.T) {
	a := NewNoopAdapter()
	devs, err := a.ListDevices(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if devs != nil {
		t.Fatalf("expected nil slice, got %v", devs)
	}
}
