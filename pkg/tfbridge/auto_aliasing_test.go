package tfbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// Cover the RunningProviderStage fast-track of MustApplyAutoAliases. The only information needed in
// this case is autoSettings. Verify that it gets applied.
func TestFastAutoAliasingAtRuntime(t *testing.T) {
	originalStage := currentRuntimeStage
	currentRuntimeStage = runningProviderStage
	t.Cleanup(func() { currentRuntimeStage = originalStage })

	autos := autoSettings{
		Resources: map[string]*autoResourceSettings{
			"prov_r1": {
				Aliases: []tokens.Type{"pkg:mod/r1:R1A"},
			},
		},
	}

	meta := NewProviderMetadata([]byte(`{}`))
	err := md.Set(meta.Data, autoSettingsKey, autos)
	require.NoError(t, err)

	sp := &schema.Provider{
		ResourcesMap: schema.ResourceMap{
			"prov_r1": (&schema.Resource{}).Shim(),
		},
	}

	prov := &ProviderInfo{
		P:       sp.Shim(),
		Name:    "prov",
		Version: "1.0.0",
		Resources: map[string]*ResourceInfo{
			"prov_r1": {Tok: "pkg:mod/r1:R1"},
		},
		MetadataInfo: meta,
	}

	prov.MustApplyAutoAliases()

	_, ok := sp.ResourcesMap.GetOk("prov_r1_legacy")
	assert.Truef(t, ok, "Expected prov_r1 to be aliased as prov_r1_legacy by MustApplyAutoAliases")
}
