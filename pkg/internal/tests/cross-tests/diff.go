package crosstests

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/zclconf/go-cty/cty"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
)

type diffOpts struct {
	deleteBeforeReplace           bool
	disableAccurateBridgePreviews bool
}

// An option that can be used to customize [Diff].
type DiffOption func(*diffOpts)

// DiffDeleteBeforeReplace specifies whether to delete the resource before replacing it.
func DiffDeleteBeforeReplace(deleteBeforeReplace bool) DiffOption {
	return func(o *diffOpts) { o.deleteBeforeReplace = deleteBeforeReplace }
}

// DiffDisableAccurateBridgePreviews specifies whether to disable accurate bridge previews.
func DiffDisableAccurateBridgePreviews() DiffOption {
	return func(o *diffOpts) { o.disableAccurateBridgePreviews = true }
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
		Resource:            resource,
		Config1:             config1Cty,
		Config2:             config2Cty,
		DeleteBeforeReplace: o.deleteBeforeReplace,
	})
}
