package crosstests

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/internal/pulcheck"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		t.Errorf("Expected __meta to be present in the state")
		return -1
	}
	meta := metaVar.(string)
	var metaMap map[string]interface{}
	err = json.Unmarshal([]byte(meta), &metaMap)
	require.NoError(t, err)
	schemaVersion, err := strconv.ParseInt(metaMap["schema_version"].(string), 10, 64)
	require.NoError(t, err)
	return int(schemaVersion)
}

func runPulumiUpgrade(t T, res1, res2 *schema.Resource, config any, disablePlanResourceChange bool) {
	opts := []pulcheck.BridgedProviderOpt{}
	if disablePlanResourceChange {
		opts = append(opts, pulcheck.DisablePlanResourceChange())
	}

	prov1 := pulcheck.BridgedProvider(t, defProviderShortName, map[string]*schema.Resource{defRtype: res1}, opts...)
	prov2 := pulcheck.BridgedProvider(t, defProviderShortName, map[string]*schema.Resource{defRtype: res2}, opts...)

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
		objectType:          nil,
	}

	yamlProgram := pd.generateYAML(t, prov1.P.ResourcesMap(), config)
	pt := pulcheck.PulCheck(t, prov1, string(yamlProgram))

	pt.Up()
	stack := pt.ExportStack()
	schemaVersion := getVersionInState(t, stack)
	assert.Equal(t, res1.SchemaVersion, schemaVersion)

	handle, err := pulcheck.StartPulumiProvider(context.Background(), defProviderShortName, defProviderVer, prov2)
	require.NoError(t, err)
	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS", fmt.Sprintf("%s:%d", defProviderShortName, handle.Port))
	pt.Up()
	stack = pt.ExportStack()
	schemaVersion = getVersionInState(t, stack)
	assert.Equal(t, res2.SchemaVersion, schemaVersion)
}

func runUpgradeStateInputCheck(t T, tc inputTestCase) {
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

	tfd := newTfDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource)
	_ = tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tc.Config)

	tfd2 := newTfDriver(t, tfwd, defProviderShortName, defRtype, &upgradeRes)
	_ = tfd2.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tc.Config)

	runPulumiUpgrade(t, tc.Resource, &upgradeRes, tc.Config, tc.DisablePlanResourceChange)

	assert.Len(t, upgradeRawStates, 2)
	if len(upgradeRawStates) != 2 {
		return
	}
	assertValEqual(t, "UpgradeRawState", upgradeRawStates[0], upgradeRawStates[1])
}
