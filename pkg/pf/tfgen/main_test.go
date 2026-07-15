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

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// These tests exercise Main and MainWithMuxer end to end, including the
// os.Exit(-1) path, by re-executing the current test binary as a subprocess.
// The subprocess overrides os.Args itself before invoking Main/MainWithMuxer,
// so there is no interference from `go test` flags.

const helperEnvVar = "PFTFGEN_MAIN_TEST_HELPER"

func runMainHelperSubprocess(t *testing.T, testName string, env ...string) (stdout, stderr string, exitErr error) {
	t.Helper()

	// The test binary itself is re-executed with a test-selector flag; testName is
	// always a hardcoded literal from a call site in this file, not external input.
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$", "-test.v=false") //nolint:gosec
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, helperEnvVar+"=1")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	exitErr = cmd.Run()
	return outBuf.String(), errBuf.String(), exitErr
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

func TestMainRejectsEmptyVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(helperEnvVar) == "1" {
		os.Args = []string{"pulumi-tfgen-test", "schema"}
		Main("testprovider", tfbridge.ProviderInfo{
			Name:         "testprovider",
			P:            pftfbridge.ShimProvider(minimalPFResourceProvider()),
			Version:      "",
			MetadataInfo: tfbridge.NewProviderMetadata([]byte("{}")),
		})
		return
	}

	_, stderr, err := runMainHelperSubprocess(t, "TestMainRejectsEmptyVersion")
	require.Error(t, err, "Main should exit with a non-zero status for an empty version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
}

func TestMainRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(helperEnvVar) == "1" {
		os.Args = []string{"pulumi-tfgen-test", "schema"}
		Main("testprovider", tfbridge.ProviderInfo{
			Name:         "testprovider",
			P:            pftfbridge.ShimProvider(minimalPFResourceProvider()),
			Version:      "not-a-version",
			MetadataInfo: tfbridge.NewProviderMetadata([]byte("{}")),
		})
		return
	}

	_, stderr, err := runMainHelperSubprocess(t, "TestMainRejectsInvalidVersion")
	require.Error(t, err, "Main should exit with a non-zero status for an invalid version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
	require.Contains(t, stderr, "not-a-version")
}

func TestMainAcceptsValidVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(helperEnvVar) == "1" {
		outDir, err := os.MkdirTemp("", "pftfgen-main-test")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(outDir)

		os.Args = []string{
			"pulumi-tfgen-test", "schema",
			"--out", outDir,
			"--skip-docs",
			"--skip-examples",
		}
		Main("testprovider", tfbridge.ProviderInfo{
			Name:         "testprovider",
			P:            pftfbridge.ShimProvider(minimalPFResourceProvider()),
			Version:      "1.2.3",
			MetadataInfo: tfbridge.NewProviderMetadata([]byte("{}")),
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_thing": {Tok: "testprovider:index:Thing"},
			},
		})
		return
	}

	stdout, stderr, err := runMainHelperSubprocess(t, "TestMainAcceptsValidVersion")
	require.NoError(t, err, "Main should succeed for a valid version: stdout=%s stderr=%s", stdout, stderr)
	require.NotContains(t, stderr, "ProviderInfo.Version is required")
}

func TestMainWithMuxerRejectsEmptyVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(helperEnvVar) == "1" {
		os.Args = []string{"pulumi-tfgen-test", "schema"}
		ctx := context.Background()
		sdkProvider := &sdkschema.Provider{
			Schema:       map[string]*sdkschema.Schema{},
			ResourcesMap: map[string]*sdkschema.Resource{},
		}
		MainWithMuxer("testprovider", tfbridge.ProviderInfo{
			Name:         "testprovider",
			P:            pftfbridge.MuxShimWithPF(ctx, sdkv2shim.NewProvider(sdkProvider), minimalPFResourceProvider()),
			Version:      "",
			MetadataInfo: tfbridge.NewProviderMetadata([]byte("{}")),
		})
		return
	}

	_, stderr, err := runMainHelperSubprocess(t, "TestMainWithMuxerRejectsEmptyVersion")
	require.Error(t, err, "MainWithMuxer should exit with a non-zero status for an empty version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
}

func TestMainWithMuxerRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(helperEnvVar) == "1" {
		os.Args = []string{"pulumi-tfgen-test", "schema"}
		ctx := context.Background()
		sdkProvider := &sdkschema.Provider{
			Schema:       map[string]*sdkschema.Schema{},
			ResourcesMap: map[string]*sdkschema.Resource{},
		}
		MainWithMuxer("testprovider", tfbridge.ProviderInfo{
			Name:         "testprovider",
			P:            pftfbridge.MuxShimWithPF(ctx, sdkv2shim.NewProvider(sdkProvider), minimalPFResourceProvider()),
			Version:      "not-a-version",
			MetadataInfo: tfbridge.NewProviderMetadata([]byte("{}")),
		})
		return
	}

	_, stderr, err := runMainHelperSubprocess(t, "TestMainWithMuxerRejectsInvalidVersion")
	require.Error(t, err, "MainWithMuxer should exit with a non-zero status for an invalid version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
	require.Contains(t, stderr, "not-a-version")
}
