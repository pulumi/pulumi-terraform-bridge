package tfbridge

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// Cover the RunningProviderStage fast-track of MustApplyAutoAliases. The only information needed in
// this case is autoSettings. Verify that it gets applied.
func TestFastAutoAliasingAtRuntime(t *testing.T) {
	t.Parallel()
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

// A resource renamed more than once in the same major version has several past names. Each of
// them must be kept as an alias of the current resource.
func TestAutoResourceSettingsApplyMultiplePastNames(t *testing.T) {
	t.Parallel()

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
	}

	autoResourceSettings{
		Aliases: []tokens.Type{"pkg:mod/r1:R1A", "pkg:mod/r1:R1B"},
	}.apply(prov, "prov_r1")

	var aliases []string
	for _, a := range prov.Resources["prov_r1"].Aliases {
		aliases = append(aliases, *a.Type)
	}
	assert.Equal(t, []string{"pkg:mod/r1:R1A", "pkg:mod/r1:R1B"}, aliases)

	// The legacy resource keeps pointing at the last past name, as it did before multiple past names were kept.
	assert.Equal(t, tokens.Type("pkg:mod/r1:R1B"), prov.Resources["prov_r1_legacy"].Tok)
}
