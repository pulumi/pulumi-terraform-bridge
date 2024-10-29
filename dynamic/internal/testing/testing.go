package testing

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func Integration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skipping integration test during -short")
	}
}

func BuildOnce(globalTempDir *string, dir, name string) func(t *testing.T) string {
	mkBin := sync.OnceValues(func() (string, error) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}

		out := filepath.Join(*globalTempDir, name)
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = filepath.Join(wd, dir)
		stdoutput, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to build provider: %w:\n%s", err, string(stdoutput))
		}
		return out, nil
	})

	return func(t *testing.T) string {
		t.Helper()
		path, err := mkBin()
		require.NoErrorf(t, err, "failed find provider path")
		return path
	}
}
