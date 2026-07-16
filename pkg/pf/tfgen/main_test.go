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

package tfgen

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// These tests exercise Main and MainWithMuxer end to end, including the
// os.Exit(-1) path, by re-executing the current test binary as a subprocess
// that runs TestTFGenMainHelperProcess. The scenario (which entrypoint to
// call, what version to pass, and any extra environment such as
// COVERAGE_OUTPUT_DIR) is selected entirely through environment variables set
// by the parent, so there is only one child-mode branch for the whole file
// instead of one per scenario.
//
// Each subprocess is bounded by tfgenSubprocessTimeout: if a regression makes
// Main/MainWithMuxer block (e.g. validation stops happening before generation
// and something downstream hangs), the test fails fast with a clear timeout
// message instead of blocking until the repository's overall test timeout.
const (
	tfgenHelperEntrypointEnvVar = "PFTFGEN_MAIN_TEST_ENTRYPOINT" // "main" or "muxer"
	tfgenHelperVersionEnvVar    = "PFTFGEN_MAIN_TEST_VERSION"
	tfgenHelperOutDirEnvVar     = "PFTFGEN_MAIN_TEST_OUTDIR"
	tfgenSubprocessTimeout      = 30 * time.Second
)

// TestTFGenMainHelperProcess is not a real test on its own; it is invoked as a
// subprocess by TestTFGenMainAndMainWithMuxerVersionValidation below, with
// tfgenHelperEntrypointEnvVar set to select which entrypoint to call. Running
// it directly (e.g. via `go test -run`) without that variable set is a no-op.
// It intentionally does not call t.Parallel(): it is only ever run in
// isolation by runTFGenHelperProcess via -test.run, never alongside sibling
// tests in the same process.
func TestTFGenMainHelperProcess(t *testing.T) { //nolint:paralleltest
	entrypoint := os.Getenv(tfgenHelperEntrypointEnvVar)
	if entrypoint == "" {
		t.Skip("only runs as a subprocess helper selected by " + tfgenHelperEntrypointEnvVar)
	}

	os.Args = []string{
		"pulumi-tfgen-test", "schema",
		"--out", os.Getenv(tfgenHelperOutDirEnvVar),
		"--skip-docs",
		"--skip-examples",
	}
	version := os.Getenv(tfgenHelperVersionEnvVar)

	switch entrypoint {
	case "main":
		Main("testprovider", pfProviderInfo(version))
	case "muxer":
		MainWithMuxer("testprovider", muxedProviderInfo(version))
	default:
		t.Fatalf("unknown %s: %q", tfgenHelperEntrypointEnvVar, entrypoint)
	}
}

func minimalPFResourceProvider() *schemaTestProvider {
	return &schemaTestProvider{
		resources: map[string]rschema.Schema{
			"thing": {
				Attributes: map[string]rschema.Attribute{
					"id": rschema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

// pfProviderInfo builds a PF-only ProviderInfo with the given version. It is
// valid enough to complete generation when the version is valid, and lets the
// reject-version tests exercise Main all the way from a realistic call site.
func pfProviderInfo(version string) tfbridge.ProviderInfo {
	return tfbridge.ProviderInfo{
		Name:         "testprovider",
		P:            pftfbridge.ShimProvider(minimalPFResourceProvider()),
		Version:      version,
		MetadataInfo: tfbridge.NewProviderMetadata([]byte("{}")),
		Resources: map[string]*tfbridge.ResourceInfo{
			"test_thing": {Tok: "testprovider:index:Thing"},
		},
	}
}

// muxedProviderInfo builds a muxed (SDKv2 + PF) ProviderInfo with the given
// version, valid enough to complete generation when the version is valid.
func muxedProviderInfo(version string) tfbridge.ProviderInfo {
	ctx := context.Background()
	sdkProvider := &sdkschema.Provider{
		Schema:       map[string]*sdkschema.Schema{},
		ResourcesMap: map[string]*sdkschema.Resource{},
	}
	return tfbridge.ProviderInfo{
		Name:         "testprovider",
		P:            pftfbridge.MuxShimWithPF(ctx, sdkv2shim.NewProvider(sdkProvider), minimalPFResourceProvider()),
		Version:      version,
		MetadataInfo: tfbridge.NewProviderMetadata([]byte("{}")),
		Resources: map[string]*tfbridge.ResourceInfo{
			"test_thing": {Tok: "testprovider:index:Thing"},
		},
	}
}

// tfgenScenario is one row of the table driving TestTFGenMainAndMainWithMuxerVersionValidation.
type tfgenScenario struct {
	name       string
	entrypoint string // "main" or "muxer"
	version    string
	// extraEnv, if set, is called once per test run to compute additional
	// subprocess environment variables (e.g. a fresh COVERAGE_OUTPUT_DIR).
	extraEnv func(t *testing.T) []string

	wantErr        bool
	stderrContains []string
}

func TestTFGenMainAndMainWithMuxerVersionValidation(t *testing.T) {
	t.Parallel()

	const versionErrMsg = "ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible"

	scenarios := []tfgenScenario{
		{
			name:           "Main rejects an empty version",
			entrypoint:     "main",
			version:        "",
			wantErr:        true,
			stderrContains: []string{versionErrMsg},
		},
		{
			name:           "Main rejects an invalid version",
			entrypoint:     "main",
			version:        "not-a-version",
			wantErr:        true,
			stderrContains: []string{versionErrMsg, "not-a-version"},
		},
		{
			name:       "Main accepts a valid version",
			entrypoint: "main",
			version:    "1.2.3",
			wantErr:    false,
		},
		{
			// Regression test: placing the version check inside the
			// MainWithCustomGenerate callback let the error be swallowed when
			// COVERAGE_OUTPUT_DIR is set, because MainWithCustomGenerate
			// overwrites the callback's error with the coverage export
			// result. The validation must run before MainWithCustomGenerate
			// so it fails fast regardless of coverage tracking.
			name:           "Main rejects an invalid version even with coverage tracking enabled",
			entrypoint:     "main",
			version:        "not-a-version",
			wantErr:        true,
			stderrContains: []string{versionErrMsg},
			extraEnv: func(t *testing.T) []string {
				return []string{"COVERAGE_OUTPUT_DIR=" + t.TempDir()}
			},
		},
		{
			name:           "MainWithMuxer rejects an empty version",
			entrypoint:     "muxer",
			version:        "",
			wantErr:        true,
			stderrContains: []string{versionErrMsg},
		},
		{
			name:           "MainWithMuxer rejects an invalid version",
			entrypoint:     "muxer",
			version:        "not-a-version",
			wantErr:        true,
			stderrContains: []string{versionErrMsg, "not-a-version"},
		},
		{
			name:       "MainWithMuxer accepts a valid version",
			entrypoint: "muxer",
			version:    "1.2.3",
			wantErr:    false,
		},
	}

	for _, tc := range scenarios {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := []string{
				tfgenHelperEntrypointEnvVar + "=" + tc.entrypoint,
				tfgenHelperVersionEnvVar + "=" + tc.version,
				tfgenHelperOutDirEnvVar + "=" + t.TempDir(),
			}
			if tc.extraEnv != nil {
				env = append(env, tc.extraEnv(t)...)
			}

			stdout, stderr, err := runTFGenHelperProcess(t, env...)

			if tc.wantErr {
				require.Error(t, err, "expected a non-zero exit status: stdout=%s stderr=%s", stdout, stderr)
			} else {
				require.NoError(t, err, "expected success: stdout=%s stderr=%s", stdout, stderr)
			}
			for _, want := range tc.stderrContains {
				require.Contains(t, stderr, want)
			}
		})
	}
}

// runTFGenHelperProcess re-executes the current test binary, running only
// TestTFGenMainHelperProcess, with env applied on top of the current
// environment. The subprocess is bounded by tfgenSubprocessTimeout so a
// hanging child (e.g. Main/MainWithMuxer unexpectedly blocking) fails fast
// with a clear diagnostic instead of running until the suite-wide timeout.
func runTFGenHelperProcess(t *testing.T, env ...string) (stdout, stderr string, exitErr error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), tfgenSubprocessTimeout)
	defer cancel()

	// The test binary itself is re-executed selecting TestTFGenMainHelperProcess;
	// scenario selection happens entirely through the fixed env vars set by the
	// caller above, not through any externally-influenced argument.
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTFGenMainHelperProcess$", "-test.v=false") //nolint:gosec
	cmd.Env = append(os.Environ(), env...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	exitErr = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("subprocess timed out after %s (stdout=%s stderr=%s): %v",
			tfgenSubprocessTimeout, outBuf.String(), errBuf.String(), ctx.Err())
	}
	return outBuf.String(), errBuf.String(), exitErr
}
