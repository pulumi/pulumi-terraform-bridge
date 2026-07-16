// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridge

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// These tests exercise the runtime Main and MainWithMuxer entrypoints,
// including the os.Exit path, by re-executing the current test binary as a
// subprocess that runs TestTFBridgeMainHelperProcess. The scenario (which
// entrypoint to call and what version to pass) is selected entirely through
// environment variables set by the parent, so there is only one child-mode
// branch for the whole file instead of one per scenario.
//
// Using `--version` on a valid ProviderInfo makes handleFlags print the
// version and exit(0) before ever reaching serve(), so these tests never
// block on an actual provider RPC server. As a second line of defense, each
// subprocess is also bounded by tfbridgeSubprocessTimeout: if that assumption
// ever regresses, the test fails fast with a clear timeout message instead of
// blocking until the repository's overall test timeout.
const (
	tfbridgeHelperEntrypointEnvVar = "PFTFBRIDGE_MAIN_TEST_ENTRYPOINT" // "main" or "muxer"
	tfbridgeHelperVersionEnvVar    = "PFTFBRIDGE_MAIN_TEST_VERSION"
	tfbridgeSubprocessTimeout      = 30 * time.Second
)

// TestTFBridgeMainHelperProcess is not a real test on its own; it is invoked
// as a subprocess by TestTFBridgeMainAndMainWithMuxerVersionValidation below,
// with tfbridgeHelperEntrypointEnvVar set to select which entrypoint to call.
// Running it directly (e.g. via `go test -run`) without that variable set is
// a no-op. It intentionally does not call t.Parallel(): it is only ever run
// in isolation by runTFBridgeHelperProcess via -test.run, never alongside
// sibling tests in the same process.
func TestTFBridgeMainHelperProcess(t *testing.T) { //nolint:paralleltest
	entrypoint := os.Getenv(tfbridgeHelperEntrypointEnvVar)
	if entrypoint == "" {
		t.Skip("only runs as a subprocess helper selected by " + tfbridgeHelperEntrypointEnvVar)
	}

	os.Args = []string{"pulumi-resource-test", "--version"}
	version := os.Getenv(tfbridgeHelperVersionEnvVar)
	info := tfbridge.ProviderInfo{
		Name:    "testprovider",
		Version: version,
	}

	switch entrypoint {
	case "main":
		Main(context.Background(), "testprovider", info, ProviderMetadata{PackageSchema: []byte(`{}`)})
	case "muxer":
		MainWithMuxer(context.Background(), "testprovider", info, []byte(`{}`))
	default:
		t.Fatalf("unknown %s: %q", tfbridgeHelperEntrypointEnvVar, entrypoint)
	}
}

// tfbridgeScenario is one row of the table driving
// TestTFBridgeMainAndMainWithMuxerVersionValidation.
type tfbridgeScenario struct {
	name       string
	entrypoint string // "main" or "muxer"
	version    string

	wantErr         bool
	stderrContains  []string
	stdoutContains  []string
	wantEmptyStdout bool
}

func TestTFBridgeMainAndMainWithMuxerVersionValidation(t *testing.T) {
	t.Parallel()

	const versionErrMsg = "ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible"

	scenarios := []tfbridgeScenario{
		{
			name:            "Main rejects an empty version",
			entrypoint:      "main",
			version:         "",
			wantErr:         true,
			stderrContains:  []string{versionErrMsg},
			wantEmptyStdout: true, // must not reach --version handling
		},
		{
			name:            "Main rejects an invalid version",
			entrypoint:      "main",
			version:         "not-a-version",
			wantErr:         true,
			stderrContains:  []string{versionErrMsg, "not-a-version"},
			wantEmptyStdout: true, // must not reach --version handling
		},
		{
			name:           "Main accepts a valid version",
			entrypoint:     "main",
			version:        "1.2.3",
			wantErr:        false,
			stdoutContains: []string{"1.2.3"},
		},
		{
			name:            "MainWithMuxer rejects an empty version",
			entrypoint:      "muxer",
			version:         "",
			wantErr:         true,
			stderrContains:  []string{versionErrMsg},
			wantEmptyStdout: true, // must not reach --version handling
		},
		{
			name:            "MainWithMuxer rejects an invalid version",
			entrypoint:      "muxer",
			version:         "not-a-version",
			wantErr:         true,
			stderrContains:  []string{versionErrMsg, "not-a-version"},
			wantEmptyStdout: true, // must not reach --version handling
		},
		{
			name:           "MainWithMuxer accepts a valid version",
			entrypoint:     "muxer",
			version:        "1.2.3",
			wantErr:        false,
			stdoutContains: []string{"1.2.3"},
		},
	}

	for _, tc := range scenarios {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := []string{
				tfbridgeHelperEntrypointEnvVar + "=" + tc.entrypoint,
				tfbridgeHelperVersionEnvVar + "=" + tc.version,
			}

			stdout, stderr, err := runTFBridgeHelperProcess(t, env...)

			if tc.wantErr {
				require.Error(t, err, "expected a non-zero exit status: stdout=%s stderr=%s", stdout, stderr)
			} else {
				require.NoError(t, err, "expected success: stdout=%s stderr=%s", stdout, stderr)
			}
			for _, want := range tc.stderrContains {
				require.Contains(t, stderr, want)
			}
			for _, want := range tc.stdoutContains {
				require.Contains(t, stdout, want)
			}
			if tc.wantEmptyStdout {
				require.Empty(t, stdout, "should not reach --version handling")
			}
		})
	}
}

// runTFBridgeHelperProcess re-executes the current test binary, running only
// TestTFBridgeMainHelperProcess, with env applied on top of the current
// environment. The subprocess is bounded by tfbridgeSubprocessTimeout so a
// hanging child (e.g. Main/MainWithMuxer unexpectedly reaching serve()
// instead of exiting on --version) fails fast with a clear diagnostic instead
// of running until the suite-wide timeout.
func runTFBridgeHelperProcess(t *testing.T, env ...string) (stdout, stderr string, exitErr error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), tfbridgeSubprocessTimeout)
	defer cancel()

	// The test binary itself is re-executed selecting TestTFBridgeMainHelperProcess;
	// scenario selection happens entirely through the fixed env vars set by the
	// caller above, not through any externally-influenced argument.
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTFBridgeMainHelperProcess$") //nolint:gosec
	cmd.Env = append(os.Environ(), env...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	exitErr = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("subprocess timed out after %s (stdout=%s stderr=%s): %v",
			tfbridgeSubprocessTimeout, outBuf.String(), errBuf.String(), ctx.Err())
	}
	return outBuf.String(), errBuf.String(), exitErr
}
