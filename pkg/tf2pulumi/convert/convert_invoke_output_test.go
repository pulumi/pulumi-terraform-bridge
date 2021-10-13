// Copyright 2016-2021, Pulumi Corporation.
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

package convert

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/spf13/afero"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertInvokeOutput(t *testing.T) {
	inputFile := "fun_example.hcl"

	inputBytes, err := ioutil.ReadFile(inputFile)
	require.NoError(t, err)

	originalHCL := string(inputBytes)

	files, diags, err := Convert(Options{
		Root:                   buildHCLFileSystem(t, "fun_example.tf", originalHCL),
		TargetLanguage:         "csharp",
		AllowMissingProperties: true,
		AllowMissingVariables:  true,
		FilterResourceNames:    true,
		// Logger:                   logger,
		// PackageCache:             g.packageCache,
		// PluginHost:               g.pluginHost,
		// ProviderInfoSource:       g.infoSource,
		SkipResourceTypechecking: true,
		// TerraformVersion:         g.terraformVersion,
		SourceHCL:      originalHCL,
		JobDescription: "testjob",
	})

	require.NoError(t, err)

	t.Logf("Files:")
	for f, bytes := range files {
		t.Logf("File %s: %v", f, string(bytes))
	}
	t.Logf("Diags: %v", diags)

}

func buildHCLFileSystem(t *testing.T, path string, hcl string) afero.Fs {
	input := afero.NewMemMapFs()
	f, err := input.Create(fmt.Sprintf("/%s.tf", strings.ReplaceAll(path, "/", "-")))
	defer require.NoError(t, f.Close())
	require.NoError(t, err)
	_, err = f.Write([]byte(hcl))
	require.NoError(t, err)
	return input
}
