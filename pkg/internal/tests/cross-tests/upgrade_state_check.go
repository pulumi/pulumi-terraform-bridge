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

//nolint:lll
package crosstests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"gotest.tools/v3/assert"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
)

type upgradeStateTestCase struct {
	// Schema for the resource under test
	Resource *schema.Resource

	Config1     any
	Config2     any
	ExpectEqual bool
	ObjectType  *tftypes.Object
}

func getVersionInState(t T, stack apitype.UntypedDeployment) int {
	data, err := stack.Deployment.MarshalJSON()
	require.NoError(t, err)

	var stateMap map[string]interface{}
	err = json.Unmarshal(data, &stateMap)
	require.NoError(t, err)

	resourcesList := stateMap["resources"].([]interface{})
	require.Len(t, resourcesList, 3)
	testResState := resourcesList[2].(map[string]interface{})
	resOutputs := testResState["outputs"].(map[string]interface{})
	metaVar := resOutputs["__meta"]
	if metaVar == nil {
		// If the resource does not have a meta field, assume the schema version is 0.
		return 0
	}
	meta := metaVar.(string)
	var metaMap map[string]interface{}
	err = json.Unmarshal([]byte(meta), &metaMap)
	require.NoError(t, err)
	schemaVersion, err := strconv.ParseInt(metaMap["schema_version"].(string), 10, 64)
	require.NoError(t, err)
	return int(schemaVersion)
}

func runPulumiUpgrade(t T, res1, res2 *schema.Resource, config1, config2 cty.Value) (int, int) {
	opts := []pulcheck.BridgedProviderOpt{}

	tfp1 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: res1}}
	prov1 := pulcheck.BridgedProvider(t, defProviderShortName, tfp1, opts...)
	tfp2 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: res2}}
	prov2 := pulcheck.BridgedProvider(t, defProviderShortName, tfp2, opts...)

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
	}

	yamlProgram := pd.generateYAML(t, crosstestsimpl.InferPulumiValue(t,
		prov1.P.ResourcesMap().Get(pd.tfResourceName).Schema(), nil, config1))
	pt := pulcheck.PulCheck(t, prov1, string(yamlProgram))
	pt.Up(t)
	stack := pt.ExportStack(t)
	schemaVersion1 := getVersionInState(t, stack)

	yamlProgram = pd.generateYAML(t, crosstestsimpl.InferPulumiValue(t,
		prov1.P.ResourcesMap().Get(pd.tfResourceName).Schema(), nil, config2))
	p := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
	err := os.WriteFile(p, yamlProgram, 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")

	handle, err := pulcheck.StartPulumiProvider(context.Background(), defProviderShortName, defProviderVer, prov2)
	require.NoError(t, err)
	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS", fmt.Sprintf("%s:%d", defProviderShortName, handle.Port))
	pt.Up(t)

	stack = pt.ExportStack(t)
	schemaVersion2 := getVersionInState(t, stack)

	return schemaVersion1, schemaVersion2
}

func runUpgradeStateInputCheck(t T, tc upgradeStateTestCase) {
	upgrades := make([]schema.StateUpgrader, 0)
	for i := 0; i < tc.Resource.SchemaVersion; i++ {
		upgrades = append(upgrades, schema.StateUpgrader{
			Version: i,
			Type:    tc.Resource.CoreConfigSchema().ImpliedType(),
			Upgrade: func(
				ctx context.Context,
				rawState map[string]interface{},
				meta interface{},
			) (map[string]interface{}, error) {
				return rawState, nil
			},
		})
	}

	upgradeRawStates := make([]map[string]interface{}, 0)

	upgrades = append(upgrades,
		schema.StateUpgrader{
			Version: tc.Resource.SchemaVersion,
			Type:    tc.Resource.CoreConfigSchema().ImpliedType(),
			Upgrade: func(
				ctx context.Context,
				rawState map[string]interface{},
				meta interface{},
			) (map[string]interface{}, error) {
				upgradeRawStates = append(upgradeRawStates, rawState)
				return rawState, nil
			},
		},
	)

	upgradeRes := *tc.Resource
	upgradeRes.SchemaVersion = upgradeRes.SchemaVersion + 1
	upgradeRes.StateUpgraders = upgrades

	tfwd := t.TempDir()

	config1 := coalesceInputs(t, tc.Resource.Schema, tc.Config1)
	config2 := coalesceInputs(t, tc.Resource.Schema, tc.Config2)

	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource)
	_ = tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", config1, lifecycleArgs{})

	tfd2 := newTFResDriver(t, tfwd, defProviderShortName, defRtype, &upgradeRes)
	_ = tfd2.writePlanApply(t, tc.Resource.Schema, defRtype, "example", config2, lifecycleArgs{})

	schemaVersion1, schemaVersion2 := runPulumiUpgrade(t, tc.Resource, &upgradeRes, config1, config2)

	if tc.ExpectEqual {
		assert.Equal(t, schemaVersion1, tc.Resource.SchemaVersion)
		// We never upgrade the state to the new version.
		// TODO: should we?

		require.Len(t, upgradeRawStates, 2)
		if len(upgradeRawStates) != 2 {
			return
		}
		assertValEqual(t, "UpgradeRawState", upgradeRawStates[0], upgradeRawStates[1])
	} else {
		assert.Equal(t, schemaVersion1, tc.Resource.SchemaVersion)
		assert.Equal(t, schemaVersion2, upgradeRes.SchemaVersion)
		require.Len(t, upgradeRawStates, 4)
		if len(upgradeRawStates) != 4 {
			return
		}
		assertValEqual(t, "UpgradeRawState", upgradeRawStates[0], upgradeRawStates[2])
		assertValEqual(t, "UpgradeRawState", upgradeRawStates[1], upgradeRawStates[3])
	}
}
