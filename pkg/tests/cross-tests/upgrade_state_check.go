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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

type upgradeStateTestCase struct {
	// Schema for the resource under test
	Resource *schema.Resource

	Config1     any
	Config2     any
	ExpectEqual bool
	ObjectType  *tftypes.Object

	DisablePlanResourceChange bool
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

func runPulumiUpgrade(t T, res1, res2 *schema.Resource, config1, config2 any, disablePlanResourceChange bool) (int, int) {
	opts := []pulcheck.BridgedProviderOpt{}
	if disablePlanResourceChange {
		opts = append(opts, pulcheck.DisablePlanResourceChange())
	}

	tfp1 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: res1}}
	prov1 := pulcheck.BridgedProvider(t, defProviderShortName, tfp1, opts...)
	tfp2 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: res2}}
	prov2 := pulcheck.BridgedProvider(t, defProviderShortName, tfp2, opts...)

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
		objectType:          nil,
	}

	yamlProgram := pd.generateYAML(t, prov1.P.ResourcesMap(), config1)
	pt := pulcheck.PulCheck(t, prov1, string(yamlProgram))
	pt.Up()
	stack := pt.ExportStack()
	schemaVersion1 := getVersionInState(t, stack)

	yamlProgram = pd.generateYAML(t, prov2.P.ResourcesMap(), config2)
	p := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
	err := os.WriteFile(p, yamlProgram, 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")

	handle, err := pulcheck.StartPulumiProvider(context.Background(), defProviderShortName, defProviderVer, prov2)
	require.NoError(t, err)
	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS", fmt.Sprintf("%s:%d", defProviderShortName, handle.Port))
	pt.Up()

	stack = pt.ExportStack()
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

	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource)
	_ = tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tc.Config1, lifecycleArgs{})

	tfd2 := newTFResDriver(t, tfwd, defProviderShortName, defRtype, &upgradeRes)
	_ = tfd2.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tc.Config2, lifecycleArgs{})

	schemaVersion1, schemaVersion2 := runPulumiUpgrade(t, tc.Resource, &upgradeRes, tc.Config1, tc.Config2, tc.DisablePlanResourceChange)

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
