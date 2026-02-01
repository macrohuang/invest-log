package investlog

import "testing"

func TestContainsHelper(t *testing.T) {
	items := []string{"a", "b", "c"}
	if !contains(items, "b") {
		t.Fatalf("expected contains true")
	}
	if contains(items, "d") {
		t.Fatalf("expected contains false")
	}
}
