package crosstests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/internal/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/stretchr/testify/require"
)

type schemaChangeTestCase struct {
	Resource1 *schema.Resource
	Resource2 *schema.Resource

	Config1 any
	Config2 any

	DisablePlanResourceChange bool
}

func replaceYAMLProgram(t T, pt *pulumitest.PulumiTest, program []byte) {
	p := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
	err := os.WriteFile(p, program, 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")
}

func replacePulumiProvider(t T, pt *pulumitest.PulumiTest, provider info.Provider) {
	handle, err := pulcheck.StartPulumiProvider(context.Background(), defProviderShortName, defProviderVer, provider)
	require.NoError(t, err)
	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS", fmt.Sprintf("%s:%d", defProviderShortName, handle.Port))
}

func runSchemaChangeCheck(t T, tc schemaChangeTestCase) {
	tfwd := t.TempDir()
	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource1)
	_ = tfd.writePlanApply(t, tc.Resource1.Schema, defRtype, "example", tc.Config1)
	tfd2 := newTFResDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource2)
	plan := tfd2.writePlanApply(t, tc.Resource2.Schema, defRtype, "example", tc.Config2)
	tfAction := tfd.parseChangesFromTFPlan(*plan)

	opts := []pulcheck.BridgedProviderOpt{}
	if tc.DisablePlanResourceChange {
		opts = append(opts, pulcheck.DisablePlanResourceChange())
	}

	tfp1 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: tc.Resource1}}
	prov1 := pulcheck.BridgedProvider(t, defProviderShortName, tfp1, opts...)
	tfp2 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: tc.Resource2}}
	prov2 := pulcheck.BridgedProvider(t, defProviderShortName, tfp2, opts...)

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
		objectType:          nil,
	}

	yamlProgram := pd.generateYAML(t, prov1.P.ResourcesMap(), tc.Config1)
	pt := pulcheck.PulCheck(t, prov1, string(yamlProgram))
	pt.Up()

	yamlProgram = pd.generateYAML(t, prov2.P.ResourcesMap(), tc.Config2)
	replaceYAMLProgram(t, pt, yamlProgram)
	replacePulumiProvider(t, pt, prov2)

	res := pt.Up()

	verifyBasicDiffAgreement(t, tfAction, res.Summary)
}