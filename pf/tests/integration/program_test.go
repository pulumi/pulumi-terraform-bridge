// Copyright 2016-2022, Pulumi Corporation.
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

package itests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"sort"
)

func TestBasicProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a PATH setup issue where the test cannot find pulumi-resource-testbridge.exe")
	}

	wd, err := os.Getwd()
	assert.NoError(t, err)
	bin := filepath.Join(wd, "..", "bin")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Env: []string{fmt.Sprintf("PATH=%s", bin)},
		Dir: filepath.Join("..", "testdata", "basicprogram"),
		PrepareProject: func(info *engine.Projinfo) error {
			return prepareStateFolder(info.Root)
		},
		ExtraRuntimeValidation: validateExpectedVsActual,
	})
}

func TestUpdateProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a PATH setup issue where the test cannot find pulumi-resource-testbridge.exe")
	}

	wd, err := os.Getwd()
	assert.NoError(t, err)
	bin := filepath.Join(wd, "..", "bin")

	editDirs := func(edits ...integration.EditDir) []integration.EditDir {
		for i, edit := range edits {
			edit.Dir = filepath.Join("..", "testdata", fmt.Sprintf("updateprogram-%d", i+2))
			edit.Additive = true
			edits[i] = edit
		}
		return edits
	}

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Env: []string{fmt.Sprintf("PATH=%s", bin)},
		Dir: filepath.Join("..", "testdata", "updateprogram"),
		PrepareProject: func(info *engine.Projinfo) error {
			return prepareStateFolder(info.Root)
		},
		EditDirs: editDirs(
			integration.EditDir{
				ExtraRuntimeValidation: validateExpectedVsActual,
			},
			integration.EditDir{
				ExpectFailure: true,
			},
			integration.EditDir{
				ExtraRuntimeValidation: validateExpectedVsActual,
			},
		),
		ExtraRuntimeValidation: validateExpectedVsActual,
	})
}

func TestDefaultInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a PATH setup issue where the test cannot find pulumi-resource-testbridge.exe")
	}

	wd, err := os.Getwd()
	assert.NoError(t, err)
	bin := filepath.Join(wd, "..", "bin")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Env: []string{fmt.Sprintf("PATH=%s", bin)},
		Dir: filepath.Join("..", "testdata", "defaultinfo-program"),
		PrepareProject: func(info *engine.Projinfo) error {
			return prepareStateFolder(info.Root)
		},
		ExtraRuntimeValidation: validateExpectedVsActual,
	})
}

func TestPrivateState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a PATH setup issue where the test cannot find pulumi-resource-testbridge.exe")
	}

	wd, err := os.Getwd()
	assert.NoError(t, err)
	bin := filepath.Join(wd, "..", "bin")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Env:                    []string{fmt.Sprintf("PATH=%s", bin)},
		Dir:                    filepath.Join("..", "testdata", "privst-program"),
		ExtraRuntimeValidation: validateExpectedVsActual,
		SkipRefresh:            true,
		RequireService:         true, // EditDirs do not work otherwise
		EditDirs: []integration.EditDir{
			{
				Dir:                    filepath.Join("..", "testdata", "privst-program", "edit-1"),
				ExtraRuntimeValidation: validateExpectedVsActual,
				Additive:               true,
			},
		},
	})
}

// Test skip_metadata_api_check example from pulumi-aws that is unusual in remapping a string prop to boolean.
func TestRegressSMAC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a PATH setup issue where the test cannot find pulumi-resource-testbridge.exe")
	}

	wd, err := os.Getwd()
	assert.NoError(t, err)
	bin := filepath.Join(wd, "..", "bin")

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Env:                    []string{fmt.Sprintf("PATH=%s", bin)},
		Dir:                    filepath.Join("..", "testdata", "smac-program"),
		ExtraRuntimeValidation: validateExpectedVsActual,
	})
}

func prepareStateFolder(root string) error {
	err := os.Mkdir(filepath.Join(root, "state"), 0777)
	if os.IsExist(err) {
		return nil
	}
	return err
}

func ensureTestBridgeProviderCompiled(wd string) error {
	ensure := func(exe string) error {
		cmd := exec.Command("go", "build", "-o", filepath.Join("..", "..", "..", "..", "bin", exe)) //nolint:gosec
		cmd.Dir = filepath.Join(wd, "..", "internal", "testprovider", "cmd", exe)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := ensure("pulumi-resource-testbridge"); err != nil {
		return err
	}
	return ensure("pulumi-resource-muxedrandom")

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
