package tfcheck

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
)

func TestTfComputed(t *testing.T) {
	prov := schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"computed": {
						Type:     schema.TypeString,
						Computed: true,
						Optional: true,
					},
				},
				CreateContext: func(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
					d.SetId("test")
					err := d.Set("computed", "computed")
					assert.NoError(t, err)
					return nil
				},
			},
		},
	}

	driver := NewTfDriver(t, t.TempDir(), "test", &prov)

	driver.Write(t, `
resource "test_resource" "test" {
  computed = "foo"
}
`,
	)

	plan := driver.Plan(t)
	t.Logf(plan.PlanFile)
	t.Logf(driver.Show(t, plan.PlanFile))
	driver.Apply(t, plan)

	res, err := os.ReadFile(path.Join(driver.cwd, "terraform.tfstate"))
	assert.NoError(t, err)
	t.Logf(string(res))

	newPlan := driver.Plan(t)
	t.Logf(newPlan.PlanFile)

	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, newPlan)

	res, err = os.ReadFile(path.Join(driver.cwd, "terraform.tfstate"))
	assert.NoError(t, err)
	t.Logf(string(res))
}
