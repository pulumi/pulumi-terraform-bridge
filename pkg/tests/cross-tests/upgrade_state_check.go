package crosstests

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	defProviderShortName = "crossprovider"
	defRtype             = "crossprovider_testres"
	defRtok              = "TestRes"
	defRtoken            = defProviderShortName + ":index:" + defRtok
	DefProviderVer       = "0.0.1"
)

func pulumiDriverFromRes(t T, res *schema.Resource) *pulumiDriver {
	tfp := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			defRtype: res,
		},
	}
	ensureProviderValid(t, tfp)

	shimProvider := shimv2.NewProvider(tfp, shimv2.WithPlanResourceChange(
		func(tfResourceType string) bool { return true },
	))

	return &pulumiDriver{
		name:                defProviderShortName,
		version:             DefProviderVer,
		shimProvider:        shimProvider,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
		objectType:          nil,
	}
}

func runPulumiUpgrade(t T, res1, res2 *schema.Resource, config any) {
	pd := pulumiDriverFromRes(t, res1)
	pd2 := pulumiDriverFromRes(t, res2)

	puwd := t.TempDir()
	pd.writeYAML(t, puwd, config)

	opts := []opttest.Option{
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			defProviderShortName,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle, err := pd.startPulumiProvider(ctx)
				require.NoError(t, err)
				return providers.Port(handle.Port), nil
			},
		),
	}

	pt := pulumitest.NewPulumiTest(t, puwd, opts...)

	pt.Up()

	handle, err := pd2.startPulumiProvider(context.Background())
	require.NoError(t, err)
	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS", fmt.Sprintf("%s:%d", defProviderShortName, handle.Port))
	pt.Up()
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

	runPulumiUpgrade(t, tc.Resource, &upgradeRes, tc.Config)

	assert.Len(t, upgradeRawStates, 2)
	assertValEqual(t, "UpgradeRawState", upgradeRawStates[0], upgradeRawStates[1])
}
