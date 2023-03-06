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

package tfbridgeintegrationtests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestBasicProgram(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	bin := filepath.Join(wd, "..", "bin")

	t.Run("compile-test-provider", func(t *testing.T) {
		err := ensureTestBridgeProviderCompiled(wd)
		require.NoError(t, err)
	})

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
	wd, err := os.Getwd()
	assert.NoError(t, err)
	bin := filepath.Join(wd, "..", "bin")

	t.Run("compile-test-provider", func(t *testing.T) {
		err := ensureTestBridgeProviderCompiled(wd)
		require.NoError(t, err)
	})

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Env: []string{fmt.Sprintf("PATH=%s", bin)},
		Dir: filepath.Join("..", "testdata", "updateprogram"),
		PrepareProject: func(info *engine.Projinfo) error {
			return prepareStateFolder(info.Root)
		},
		EditDirs: []integration.EditDir{
			{
				Dir:                    filepath.Join("..", "testdata", "updateprogram-2"),
				Additive:               true,
				ExtraRuntimeValidation: validateExpectedVsActual,
			},
		},
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
	exe := "pulumi-resource-testbridge"
	cmd := exec.Command("go", "build", "-o", filepath.Join("..", "..", "..", "bin", exe)) //nolint:gosec
	cmd.Dir = filepath.Join(wd, "..", "internal", "cmd", exe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
		if strings.HasSuffix(n, "__actual") {
			actuals[strings.TrimSuffix(n, "__actual")] = output
		}
		if strings.HasSuffix(n, "__expect") {
			expects[strings.TrimSuffix(n, "__expect")] = output
		}
	}
	for k := range expects {
		k := k
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, expects[k], actuals[k])
		})
	}
}
