package investlog

import "testing"

func TestStringPtrEmpty(t *testing.T) {
	if stringPtr("") != nil {
		t.Fatalf("expected nil for empty string")
	}
	val := "x"
	ptr := stringPtr(val)
	if ptr == nil || *ptr != val {
		t.Fatalf("expected pointer to %q", val)
	}
}
