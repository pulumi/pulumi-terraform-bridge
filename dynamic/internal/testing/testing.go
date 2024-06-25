package testing

import "testing"

func Integration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skipping integration test during -short")
	}
}
