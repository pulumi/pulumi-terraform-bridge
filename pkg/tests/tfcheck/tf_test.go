package tfcheck

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTfComputed(t *testing.T) {
	t.Parallel()
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

	driver := NewTfDriver(t, t.TempDir(), "test", NewTFDriverOpts{
		SDKProvider: &prov,
	})

	driver.Write(t, `
resource "test_resource" "test" {
  computed = "foo"
}
`,
	)

	plan, err := driver.Plan(t)
	require.NoError(t, err)
	t.Log(driver.Show(t, plan.PlanFile))
	err = driver.ApplyPlan(t, plan)
	require.NoError(t, err)

	t.Log(driver.GetState(t))

	newPlan, err := driver.Plan(t)
	require.NoError(t, err)

	t.Log(driver.Show(t, plan.PlanFile))

	err = driver.ApplyPlan(t, newPlan)
	require.NoError(t, err)

	t.Log(driver.GetState(t))
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
	t.Parallel()
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

	driver := NewTfDriver(t, t.TempDir(), "test", NewTFDriverOpts{
		SDKProvider: &prov,
	})

	driver.Write(t, `
resource "test_resource" "test" {
  m = {
    "string" = "123"
    "number" =  123
  }
}
`,
	)

	plan, err := driver.Plan(t)
	require.NoError(t, err)

	t.Log(driver.Show(t, plan.PlanFile))
	err = driver.ApplyPlan(t, plan)
	require.NoError(t, err)

	t.Log(driver.GetState(t))

	newPlan, err := driver.Plan(t)
	require.NoError(t, err)

	t.Log(driver.Show(t, plan.PlanFile))

	err = driver.ApplyPlan(t, newPlan)
	require.NoError(t, err)

	t.Log(driver.GetState(t))
}

func TestTfUnknownObjects(t *testing.T) {
	t.Parallel()
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

	driver := NewTfDriver(t, t.TempDir(), "test", NewTFDriverOpts{SDKProvider: &prov})

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
	plan, err := driver.Plan(t)
	require.NoError(t, err)

	t.Log(driver.Show(t, plan.PlanFile))

	err = driver.ApplyPlan(t, plan)
	require.NoError(t, err)
	t.Log(driver.GetState(t))

	driver.Write(t, unknownProgram)
	plan, err = driver.Plan(t)
	require.NoError(t, err)

	t.Log(driver.Show(t, plan.PlanFile))

	err = driver.ApplyPlan(t, plan)
	require.NoError(t, err)
	t.Log(driver.GetState(t))
}

// TF Never calls the SetHash function with nil values.
// Instead it assumes zero values for the unknowns and nils in the plan.
func TestTFSetHashNil(t *testing.T) {
	t.Parallel()

	resSch := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"bool": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"computed_bool": {
				Type:     schema.TypeBool,
				Computed: true,
				Optional: true,
			},
		},
	}

	prov := schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"set": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     resSch,
						Set: func(i interface{}) int {
							for _, v := range i.(map[string]interface{}) {
								if v == nil {
									panic("nil value in hash func")
								}
							}
							return schema.HashResource(resSch)(i)
						},
					},
				},
			},
		},
	}

	driver := NewTfDriver(t, t.TempDir(), "test", NewTFDriverOpts{SDKProvider: &prov})

	driver.Write(t, `
resource "test_resource" "test" {
	set {
		name = "foo"
	}
}
	`)

	_, err := driver.Plan(t)
	require.NoError(t, err)
}
