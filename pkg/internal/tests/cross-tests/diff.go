package crosstests

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/zclconf/go-cty/cty"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

type diffOpts struct {
	deleteBeforeReplace      bool
	resource2                *schema.Resource
	skipDiffEquivalenceCheck bool
	resourceInfo             *info.Resource
	resourceInfo2Override    *info.Resource
}

// An option that can be used to customize [Diff].
type DiffOption func(*diffOpts)

// DiffDeleteBeforeReplace specifies whether to delete the resource before replacing it.
func DiffDeleteBeforeReplace(deleteBeforeReplace bool) DiffOption {
	return func(o *diffOpts) { o.deleteBeforeReplace = deleteBeforeReplace }
}

// DiffProviderUpgradedSchema specifies the second provider schema to use for the diff.
func DiffProviderUpgradedSchema(resource2 *schema.Resource) DiffOption {
	return func(o *diffOpts) { o.resource2 = resource2 }
}

// DiffSkipDiffEquivalenceCheck specifies whether to skip the diff equivalence check.
func DiffSkipDiffEquivalenceCheck() DiffOption {
	return func(o *diffOpts) { o.skipDiffEquivalenceCheck = true }
}

// DiffResourceInfo specifies the resource info to use for the diff.
func DiffResourceInfo(resourceInfo *info.Resource) DiffOption {
	return func(o *diffOpts) { o.resourceInfo = resourceInfo }
}

// DiffResourceInfo2Override specifies an optional override for the resource info to use for the second provider.
func DiffResourceInfo2Override(resourceInfo *info.Resource) DiffOption {
	return func(o *diffOpts) { o.resourceInfo2Override = resourceInfo }
}

func Diff(
	t T, resource *schema.Resource, config1, config2 map[string]cty.Value, opts ...DiffOption,
) crosstestsimpl.DiffResult {
	o := &diffOpts{}
	for _, opt := range opts {
		opt(o)
	}

	config1Cty := cty.ObjectVal(config1)
	config2Cty := cty.ObjectVal(config2)

	return runDiffCheck(t, diffTestCase{
		Resource:                 resource,
		Config1:                  config1Cty,
		Config2:                  config2Cty,
		DeleteBeforeReplace:      o.deleteBeforeReplace,
		Resource2:                o.resource2,
		SkipDiffEquivalenceCheck: o.skipDiffEquivalenceCheck,
		ResourceInfo:             o.resourceInfo,
		ResourceInfo2Override:    o.resourceInfo2Override,
	})
}
