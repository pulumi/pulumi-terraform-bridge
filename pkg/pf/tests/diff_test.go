package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hexops/autogold/v2"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/zclconf/go-cty/cty"
)

func TestSimpleNoDiff(t *testing.T) {
	t.Parallel()

	sch := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{Optional: true},
		},
	}

	res := crosstests.Diff(t, sch,
		map[string]cty.Value{"key": cty.StringVal("value")}, map[string]cty.Value{"key": cty.StringVal("value1")})

	autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      ~ key = "value" -> "value1"
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
	autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~key]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
}
