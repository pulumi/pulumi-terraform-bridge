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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"runtime"
)

func TestMain(m *testing.M) {
	if err := setupIntegrationTests(); err != nil {
		log.Fatal(err)
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func setupIntegrationTests() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := ensureCompiledTestProviders(wd); err != nil {
		return err
	}
	return nil
}

func accTestOptions(t *testing.T) *integration.ProgramTestOptions {
	cwd, err := os.Getwd()
	if err != nil {
		t.Error("%w", err)
	}

	return &integration.ProgramTestOptions{
		Env: []string{
			fmt.Sprintf("PATH=%s", filepath.Join(cwd, "..", "..", "bin")),
		},
	}
}

func ensureCompiledTestProviders(wd string) error {
	bin := filepath.Join(wd, "..", "..", "bin")

	type testProvider struct {
		name        string
		source      string
		tfgenSource string
	}

	testProviders := []testProvider{
		{
			"tpsdkv2",
			filepath.Join(wd, "..", "..", "internal", "testprovider_sdkv2",
				"cmd", "pulumi-resource-tpsdkv2"),
			filepath.Join(wd, "..", "..", "internal", "testprovider_sdkv2",
				"cmd", "pulumi-tfgen-tpsdkv2"),
		},
	}

	runcmd := func(cmd *exec.Cmd) error {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout.String())
			fmt.Println(stderr.String())
			return err
		}
		return nil
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
			if err := runcmd(cmd); err != nil {
				return fmt.Errorf("tfgen build failed for %s: %w", p.name, err)
			}
		}

		// generate schema
		{
			exe := filepath.Join(bin, fmt.Sprintf("pulumi-tfgen-%s%s", p.name, suffix))
			cmd := exec.Command(exe, "schema", "--out", p.source)
			cmd.Dir = bin
			if err := runcmd(cmd); err != nil {
				return fmt.Errorf("schema generation failed for %s: %w", p.name, err)
			}
		}

		// build provider binary
		{
			tfgenExe := filepath.Join(bin, fmt.Sprintf("pulumi-resource-%s%s", p.name, suffix))
			cmd := exec.Command("go", "build", "-o", tfgenExe)
			cmd.Dir = p.source
			if err := runcmd(cmd); err != nil {
				return fmt.Errorf("provider build failed for %s: %w", p.name, err)
			}
		}
	}

	return nil
}
