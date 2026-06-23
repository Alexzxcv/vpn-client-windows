//go:build windows

package device

import "testing"

func TestMachineIDDeterministic(t *testing.T) {
	id1, err := MachineID()
	if err != nil {
		t.Fatalf("MachineID() first call: %v", err)
	}
	id2, err := MachineID()
	if err != nil {
		t.Fatalf("MachineID() second call: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("MachineID not deterministic: %q != %q", id1, id2)
	}
	// SHA-256 hex is 64 chars.
	if len(id1) != 64 {
		t.Fatalf("expected 64-char hex id, got %d chars: %q", len(id1), id1)
	}
}
