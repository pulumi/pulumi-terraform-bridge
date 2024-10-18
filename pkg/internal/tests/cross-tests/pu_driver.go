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

// Driver code for running tests against an in-process bridged provider under Pulumi CLI.
package crosstests

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type pulumiDriver struct {
	name                string
	pulumiResourceToken string
	tfResourceName      string
	objectType          *tftypes.Object
}

func (pd *pulumiDriver) generateYAML(t T, resMap shim.ResourceMap, tfConfig any) []byte {
	res := resMap.Get(pd.tfResourceName)
	schema := res.Schema()

	data, err := generateYaml(schema, pd.pulumiResourceToken, pd.objectType, tfConfig)
	require.NoErrorf(t, err, "generateYaml")

	b, err := yaml.Marshal(data)
	require.NoErrorf(t, err, "marshaling Pulumi.yaml")
	t.Logf("\n\n%s", b)
	return b
}
