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

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// These tests exercise the runtime Main and MainWithMuxer entrypoints,
// including the os.Exit path, by re-executing the current test binary as a
// subprocess. The subprocess overrides os.Args itself before invoking
// Main/MainWithMuxer, so there is no interference from `go test` flags.
//
// Using `--version` on a valid ProviderInfo makes handleFlags print the
// version and exit(0) before ever reaching serve(), so these tests never
// block on an actual provider RPC server.

const mainHelperEnvVar = "PFTFBRIDGE_MAIN_TEST_HELPER"

func runMainHelperSubprocess(t *testing.T, testName string, env ...string) (stdout, stderr string, exitErr error) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, mainHelperEnvVar+"=1")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	exitErr = cmd.Run()
	return outBuf.String(), errBuf.String(), exitErr
}

func TestMainRejectsEmptyVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(mainHelperEnvVar) == "1" {
		os.Args = []string{"pulumi-resource-test", "--version"}
		Main(context.Background(), "testprovider", tfbridge.ProviderInfo{
			Name:    "testprovider",
			Version: "",
		}, ProviderMetadata{PackageSchema: []byte(`{}`)})
		return
	}

	stdout, stderr, err := runMainHelperSubprocess(t, "TestMainRejectsEmptyVersion")
	require.Error(t, err, "Main should exit with a non-zero status for an empty version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
	require.Empty(t, stdout, "Main should not reach --version handling for an invalid version")
}

func TestMainRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(mainHelperEnvVar) == "1" {
		os.Args = []string{"pulumi-resource-test", "--version"}
		Main(context.Background(), "testprovider", tfbridge.ProviderInfo{
			Name:    "testprovider",
			Version: "not-a-version",
		}, ProviderMetadata{PackageSchema: []byte(`{}`)})
		return
	}

	stdout, stderr, err := runMainHelperSubprocess(t, "TestMainRejectsInvalidVersion")
	require.Error(t, err, "Main should exit with a non-zero status for an invalid version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
	require.Contains(t, stderr, "not-a-version")
	require.Empty(t, stdout, "Main should not reach --version handling for an invalid version")
}

func TestMainAcceptsValidVersionForVersionFlag(t *testing.T) {
	t.Parallel()

	if os.Getenv(mainHelperEnvVar) == "1" {
		os.Args = []string{"pulumi-resource-test", "--version"}
		Main(context.Background(), "testprovider", tfbridge.ProviderInfo{
			Name:    "testprovider",
			Version: "1.2.3",
		}, ProviderMetadata{PackageSchema: []byte(`{}`)})
		return
	}

	stdout, stderr, err := runMainHelperSubprocess(t, "TestMainAcceptsValidVersionForVersionFlag")
	require.NoError(t, err, "Main should succeed for a valid version: stdout=%s stderr=%s", stdout, stderr)
	require.Contains(t, stdout, "1.2.3")
}

func TestMainWithMuxerRejectsEmptyVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(mainHelperEnvVar) == "1" {
		os.Args = []string{"pulumi-resource-test", "--version"}
		MainWithMuxer(context.Background(), "testprovider", tfbridge.ProviderInfo{
			Name:    "testprovider",
			Version: "",
		}, []byte(`{}`))
		return
	}

	stdout, stderr, err := runMainHelperSubprocess(t, "TestMainWithMuxerRejectsEmptyVersion")
	require.Error(t, err, "MainWithMuxer should exit with a non-zero status for an empty version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
	require.Empty(t, stdout, "MainWithMuxer should not reach --version handling for an invalid version")
}

func TestMainWithMuxerRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	if os.Getenv(mainHelperEnvVar) == "1" {
		os.Args = []string{"pulumi-resource-test", "--version"}
		MainWithMuxer(context.Background(), "testprovider", tfbridge.ProviderInfo{
			Name:    "testprovider",
			Version: "not-a-version",
		}, []byte(`{}`))
		return
	}

	stdout, stderr, err := runMainHelperSubprocess(t, "TestMainWithMuxerRejectsInvalidVersion")
	require.Error(t, err, "MainWithMuxer should exit with a non-zero status for an invalid version")
	require.Contains(t, stderr,
		"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
	require.Contains(t, stderr, "not-a-version")
	require.Empty(t, stdout, "MainWithMuxer should not reach --version handling for an invalid version")
}

func TestMainWithMuxerAcceptsValidVersionForVersionFlag(t *testing.T) {
	t.Parallel()

	if os.Getenv(mainHelperEnvVar) == "1" {
		os.Args = []string{"pulumi-resource-test", "--version"}
		MainWithMuxer(context.Background(), "testprovider", tfbridge.ProviderInfo{
			Name:    "testprovider",
			Version: "1.2.3",
		}, []byte(`{}`))
		return
	}

	stdout, stderr, err := runMainHelperSubprocess(t, "TestMainWithMuxerAcceptsValidVersionForVersionFlag")
	require.NoError(t, err, "MainWithMuxer should succeed for a valid version: stdout=%s stderr=%s", stdout, stderr)
	require.Contains(t, stdout, "1.2.3")
}
