package tfcheck

import (
	"context"
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
	t.Logf(driver.Show(t, plan.PlanFile))
	driver.Apply(t, plan)

	t.Logf(driver.GetState(t))

	newPlan := driver.Plan(t)

	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, newPlan)

	t.Logf(driver.GetState(t))
}

// TestTfMapMissingElem shows that maps missing Elem types are equivalent to specifying:
//
//	Elem: &schema.Schema{Type: schema.TypeString}
//
// Previously, the bridge treated a missing map element type as `schema.TypeAny` instead
// of `schema.TypeString`, which caused provider panics. For example:
//
// - https://github.com/pulumi/pulumi-nomad/issues/389
func TestTfMapMissingElem(t *testing.T) {
	prov := schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"m": {
						Type:     schema.TypeMap,
						Required: true,
					},
				},
				CreateContext: func(ctx context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
					var errs diag.Diagnostics
					if _, ok := d.Get("m").(map[string]any)["string"].(string); !ok {
						errs = append(errs, diag.Errorf(`expected m["string"] to be a string`)...)
					}
					if _, ok := d.Get("m").(map[string]any)["number"].(string); !ok {
						errs = append(errs, diag.Errorf(`expected m["number"] to be a string`)...)
					}

					d.SetId("test")
					return errs
				},
			},
		},
	}

	driver := NewTfDriver(t, t.TempDir(), "test", &prov)

	driver.Write(t, `
resource "test_resource" "test" {
  m = {
    "string" = "123"
    "number" =  123
  }
}
`,
	)

	plan := driver.Plan(t)
	t.Logf(driver.Show(t, plan.PlanFile))
	driver.Apply(t, plan)

	t.Logf(driver.GetState(t))

	newPlan := driver.Plan(t)

	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, newPlan)

	t.Logf(driver.GetState(t))
}

func TestTfUnknownObjects(t *testing.T) {
	prov := schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"objects": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"prop": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
						Optional: true,
					},
				},
			},
			"test_aux_resource": {
				Schema: map[string]*schema.Schema{
					"objects": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"prop": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
						Optional: true,
						Computed: true,
					},
				},
				CreateContext: func(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
					d.SetId("aux")
					err := d.Set("objects", []interface{}{
						map[string]interface{}{
							"prop": "bar",
						},
					})
					assert.NoError(t, err)
					return nil
				},
			},
		},
	}

	driver := NewTfDriver(t, t.TempDir(), "test", &prov)

	knownProgram := `
resource "test_resource" "test" {
    objects {
  	    prop = "foo"
    }
}`
	unknownProgram := `
resource "test_aux_resource" "aux" {
}

resource "test_resource" "test" {
    dynamic "objects" {
	    for_each = test_aux_resource.aux.objects
        content {
            prop = objects.value["prop"]
        }
    }
}`
	driver.Write(t, knownProgram)
	plan := driver.Plan(t)
	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, plan)
	t.Logf(driver.GetState(t))

	driver.Write(t, unknownProgram)
	plan = driver.Plan(t)
	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, plan)
	t.Logf(driver.GetState(t))
}

func TestTfForceNewAdded(t *testing.T) {
	prov := schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"objects": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"prop": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
						Optional: true,
					},
				},
			},
			"test_aux_resource": {
				Schema: map[string]*schema.Schema{
					"objects": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"prop": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
						Optional: true,
						Computed: true,
					},
				},
				CreateContext: func(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
					d.SetId("aux")
					err := d.Set("objects", []interface{}{
						map[string]interface{}{
							"prop": "bar",
						},
					})
					assert.NoError(t, err)
					return nil
				},
			},
		},
	}

	driver := NewTfDriver(t, t.TempDir(), "test", &prov)

	knownProgram := `
resource "test_resource" "test" {
    objects {
  	    prop = "foo"
    }
}`
	unknownProgram := `
resource "test_aux_resource" "aux" {
}

resource "test_resource" "test" {
    dynamic "objects" {
	    for_each = test_aux_resource.aux.objects
        content {
            prop = objects.value["prop"]
        }
    }
}`
	driver.Write(t, knownProgram)
	plan := driver.Plan(t)
	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, plan)
	t.Logf(driver.GetState(t))

	driver.Write(t, unknownProgram)
	plan = driver.Plan(t)
	t.Logf(driver.Show(t, plan.PlanFile))

	driver.Apply(t, plan)
	t.Logf(driver.GetState(t))
}
