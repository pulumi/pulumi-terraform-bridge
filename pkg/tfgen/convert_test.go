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

package tfgen

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/convert"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func TestConvert(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO[pulumi/pulumi-terraform-bridge#408]
		t.Skip("Skipped on windows")
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	pluginContext, err := plugin.NewContext(
		nil, nil, nil, nil,
		cwd, nil, false, nil)
	require.NoError(t, err)
	defer contract.IgnoreClose(pluginContext)

	host := pluginContext.Host
	loader := newLoader(host)
	loader.emptyPackages["aws"] = true

	checkErr := func(hcl string) (map[string][]byte, convert.Diagnostics, error) {
		return convert.Convert(convert.Options{
			Root:                     hclToInput(t, hcl, "path"),
			TargetLanguage:           "typescript",
			AllowMissingProperties:   true,
			AllowMissingVariables:    true,
			FilterResourceNames:      true,
			Loader:                   loader,
			SkipResourceTypechecking: true,
		})
	}

	check := func(hcl string) (map[string][]byte, convert.Diagnostics) {
		files, diags, err := checkErr(hcl)
		require.NoError(t, err)
		return files, diags
	}

	t.Run("regress cannot bind expression", func(t *testing.T) {
		files, diags := check(`
                  variable "region_number" {
                    default = {
                      us-east-1 = 1
                    }
                  }`)

		generic := "<{us-east-1?: number}>"
		if isTruthy(os.Getenv("PULUMI_EXPERIMENTAL")) {
			generic = ""
		}

		expectedCode := fmt.Sprintf(`
import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const regionNumber = config.getObject%s("regionNumber") || {
    "us-east-1": 1,
};`, generic)

		require.False(t, diags.All.HasErrors())

		require.Equal(t,
			strings.TrimSpace(expectedCode),
			strings.TrimSpace(string(files["index.ts"])))
	})

	t.Run("regress no empty resource plugin found", func(t *testing.T) {
		_, _, err := checkErr(`
                  data "aws_outposts_outpost_instance_type" "example" {
                    arn                      = data.aws_outposts_outpost.example.arn
                    preferred_instance_types = ["m5.large", "m5.4xlarge"]
                  }

                  resource "aws_ec2_instance" "example" {
                    # ... other configuration ...

                    instance_type = data.aws_outposts_outpost_instance_type.example.instance_type
                  }`)
		require.Error(t, err)
	})
}

func hclToInput(t *testing.T, hcl string, path string) afero.Fs {
	// Fixup the HCL as necessary.
	if fixed, ok := fixHcl(hcl); ok {
		t.Logf("fixHcl succeeded to convert to: %s\n", fixed)
		t.Logf("fixHcl saw original hcl as %s\n", hcl)
		hcl = fixed
	} else {
		t.Logf("fixHcl left intact: %s\n", hcl)
	}

	input := afero.NewMemMapFs()
	f, err := input.Create(fmt.Sprintf("/%s.tf", strings.ReplaceAll(path, "/", "-")))
	require.NoError(t, err)
	_, err = f.Write([]byte(hcl))
	require.NoError(t, err)
	contract.IgnoreClose(f)
	return input
}
