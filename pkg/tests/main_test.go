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
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/assert"
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

	for _, p := range testProviders {
		// build tfgen binary
		{
			tfgenExe := filepath.Join(bin, fmt.Sprintf("pulumi-tfgen-%s", p.name))
			cmd := exec.Command("go", "build", "-o", tfgenExe) //nolint:gosec
			cmd.Dir = p.tfgenSource
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("tfgen build failed for %s: %w", p.name, err)
			}
		}

		// generate schema
		{
			cmd := exec.Command(filepath.Join(bin, fmt.Sprintf("pulumi-tfgen-%s", p.name)),
				"schema", "--out", p.source)
			cmd.Dir = bin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("schema generation failed for %s: %w", p.name, err)
			}
		}

		// build provider binary
		{
			tfgenExe := filepath.Join(bin, fmt.Sprintf("pulumi-resource-%s", p.name))
			cmd := exec.Command("go", "build", "-o", tfgenExe) //nolint:gosec
			cmd.Dir = p.source
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("provider build failed for %s: %w", p.name, err)
			}
		}
	}

	return nil
}

// Stacks may define tests inline by a simple convention of providing
// ${test}__expect and ${test}__actual pairs. For example:
//
//	outputs:
//	  test1__expect: 1
//	  test1__actual: ${res1.out}
//
// This function interpretes these outputs to actual tests.
func validateExpectedVsActual(t *testing.T, stack integration.RuntimeValidationStackInfo) {
	expects := map[string]interface{}{}
	actuals := map[string]interface{}{}
	for n, output := range stack.Outputs {
		switch {
		case strings.HasSuffix(n, "__actual"):
			actuals[strings.TrimSuffix(n, "__actual")] = output
		case strings.HasSuffix(n, "__expect"):
			expects[strings.TrimSuffix(n, "__expect")] = output
		case strings.HasSuffix(n, "__secret"):
			n, output := n, output
			t.Run(n, func(t *testing.T) {
				o, ok := output.(map[string]interface{})
				if assert.Truef(t, ok, "Expected Secret (map[string]any), found %T", output) {
					assert.Equal(t, "1b47061264138c4ac30d75fd1eb44270",
						o["4dabf18193072939515e22adb298388d"])
				}
			})
		}
	}
	keys := []string{}
	for k := range expects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		k := k
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, expects[k], actuals[k])
		})
	}
}
