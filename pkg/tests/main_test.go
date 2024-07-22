// Copyright 2016-2023, Pulumi Corporation.
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

package tests

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

var localTestProviders []integration.LocalDependency

func TestMain(m *testing.M) {
	if err := setupIntegrationTests(); err != nil {
		log.Fatal(err)
	}

	exitCode := m.Run()
	fmt.Fprintf(os.Stderr, "test main exited with code %d\n", exitCode)
	os.Exit(exitCode)
}

func setupIntegrationTests() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	return ensureCompiledTestProviders(wd)
}

func accTestOptions(t *testing.T) *integration.ProgramTestOptions {
	cwd, err := os.Getwd()
	if err != nil {
		t.Error("%w", err)
	}

	return &integration.ProgramTestOptions{
		Env: []string{
			fmt.Sprintf("PATH=%s", filepath.Join(cwd, "..", "..", "bin")),
			"PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true",
		},
		LocalProviders: localTestProviders,
	}
}

func ensureCompiledTestProviders(wd string) error {
	bin := filepath.Join(wd, "..", "..", "bin")

	type testProvider struct {
		name             string
		source           string
		tfgenSource      string
		expectTfgenError *string
	}

	internalErrorMsg := "Internal validation of the provider failed"

	internal := func(segments ...string) string {
		return filepath.Join(append([]string{wd, "..", "..", "internal"}, segments...)...)
	}
	testProviders := []testProvider{
		{
			"tpsdkv2",
			internal("testprovider_sdkv2", "cmd", "pulumi-resource-tpsdkv2"),
			internal("testprovider_sdkv2", "cmd", "pulumi-tfgen-tpsdkv2"),
			nil,
		},
		{
			"testprovider_invschema",
			internal("testprovider_invalid_schema", "cmd", "pulumi-resource-tpinvschema"),
			internal("testprovider_invalid_schema", "cmd", "pulumi-tfgen-tpinvschema"),
			&internalErrorMsg,
		},
	}

	runcmd := func(cmd *exec.Cmd) (error, string, string) {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return err, stdout.String(), stderr.String()
		}
		return nil, "", ""
	}

	var suffix string
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	for _, p := range testProviders {
		// build tfgen binary
		{
			tfgenExe := filepath.Join(bin, fmt.Sprintf("pulumi-tfgen-%s%s", p.name, suffix))
			cmd := exec.Command("go", "build", "-o", tfgenExe)
			cmd.Dir = p.tfgenSource
			if err, stdout, stderr := runcmd(cmd); err != nil {
				fmt.Println(stdout)
				fmt.Println(stderr)
				return fmt.Errorf("tfgen build failed for %s: %w", p.name, err)
			}
		}

		// generate schema
		{
			exe := filepath.Join(bin, fmt.Sprintf("pulumi-tfgen-%s%s", p.name, suffix))
			cmd := exec.Command(exe, "schema", "--out", p.source)
			cmd.Dir = bin
			if err, stdout, stderr := runcmd(cmd); err != nil {
				if p.expectTfgenError != nil {
					if !strings.Contains(stderr, *p.expectTfgenError) {
						fmt.Println(stdout)
						fmt.Println(stderr)
						return fmt.Errorf("tfgen schema failed for %s: %w", p.name, err)
					}
					return nil
				}
				fmt.Println(stdout)
				fmt.Println(stderr)
				return fmt.Errorf("schema generation failed for %s: %w", p.name, err)
			}
		}

		// build provider binary
		{
			tfgenExe := filepath.Join(bin, fmt.Sprintf("pulumi-resource-%s%s", p.name, suffix))
			cmd := exec.Command("go", "build", "-o", tfgenExe)
			cmd.Dir = p.source
			if err, stdout, stderr := runcmd(cmd); err != nil {
				fmt.Println(stdout)
				fmt.Println(stderr)
				return fmt.Errorf("provider build failed for %s: %w", p.name, err)
			}
			localTestProviders = append(localTestProviders, integration.LocalDependency{
				Package: p.name,
				Path:    bin, // The path to the directory that contains the binary, not the binary
			})
		}
	}

	return nil
}
