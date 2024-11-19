package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
)

func TestFullyComputedNestedAttribute(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"attached_disks": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name": {
								Optional: true,
								Type:     schema.TypeString,
							},
							"key256": {
								Computed: true,
								Type:     schema.TypeString,
							},
						},
					},
				},
				"top_level_computed": {
					Type:     schema.TypeString,
					Computed: true,
				},
			},
		},
	}

	importer := func(val any) func(context.Context, *schema.ResourceData, interface{}) ([]*schema.ResourceData, error) {
		return func(ctx context.Context, rd *schema.ResourceData, i interface{}) ([]*schema.ResourceData, error) {
			elMap := map[string]any{
				"name":   "disk1",
				"key256": val,
			}
			err := rd.Set("attached_disks", []map[string]any{elMap})
			require.NoError(t, err)

			err = rd.Set("top_level_computed", "computed_val")
			require.NoError(t, err)

			return []*schema.ResourceData{rd}, nil
		}
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	program := `
name: test
runtime: yaml
`
	for _, tc := range []struct {
		name      string
		importVal any
	}{
		{
			"non-nil",
			"val1",
		},
		{
			"nil",
			nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resMap["prov_test"].Importer = &schema.ResourceImporter{
				StateContext: importer(tc.importVal),
			}

			pt := pulcheck.PulCheck(t, bridgedProvider, program)

			res := pt.Import(t, "prov:index/test:Test", "res1", "id1", "")

			t.Log(res.Stdout)

			require.NotContains(t, res.Stdout, "One or more imported inputs failed to validate")
		})
	}
}

func TestFailedValidatorOnReadHandling(t *testing.T) {
	t.Parallel()
	type PulumiResources struct {
		Type       string                 `yaml:"type"`
		Properties map[string]interface{} `yaml:"properties"`
	}
	type PulumiYaml struct {
		Runtime   string                     `yaml:"runtime,omitempty"`
		Name      string                     `yaml:"name,omitempty"`
		Resources map[string]PulumiResources `yaml:"resources"`
	}

	tests := []struct {
		name          string
		schema        schema.Schema
		cloudVal      interface{}
		expectedProps map[string]interface{}
		expectFailure bool
	}{
		{
			name:     "TypeString no validate",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
				Optional: true,
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
		},
		{
			name:     "TypeString ValidateFunc does not error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{}
				},
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
		},
		{
			name:     "TypeString ValidateDiagFunc does not error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return nil
				},
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
		},
		{
			name:     "TypeString ValidateDiagFunc returns error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
			},
			// input dropped
			expectedProps: map[string]interface{}{},
		},
		{
			name: "TypeMap ValidateDiagFunc returns error",
			cloudVal: map[string]string{
				"nested_prop":       "ABC",
				"nested_other_prop": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			// input dropped
			expectedProps: map[string]interface{}{},
		},
		{
			name: "Non-Computed TypeMap ValidateDiagFunc does not drop",
			cloudVal: map[string]string{
				"nested_prop":       "ABC",
				"nested_other_prop": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Computed: false,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			// input not dropped
			expectedProps: map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nested_prop":       "ABC",
					"nested_other_prop": "value",
				},
			},
			// we don't drop computed: false attributes, so they will
			// still fail
			expectFailure: true,
		},
		{
			name: "Required TypeMap ValidateDiagFunc does not drop",
			cloudVal: map[string]string{
				"nested_prop":       "ABC",
				"nested_other_prop": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Required: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			expectedProps: map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nested_prop":       "ABC",
					"nested_other_prop": "value",
				},
			},
			expectFailure: true,
		},
		{
			name:     "TypeString ValidateFunc returns error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{errors.New("Error")}
				},
			},
			// input dropped
			expectedProps: map[string]interface{}{},
		},
		{
			name:     "TypeString ValidateFunc does not drop required fields",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{errors.New("Error")}
				},
			},
			expectedProps: map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			},
			expectFailure: true,
		},
		{
			name: "TypeSet ValidateDiagFunc returns error",
			cloudVal: []interface{}{
				"ABC", "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
						if val, ok := i.(string); ok && val != "ABC" {
							return diag.Errorf("Error")
						}
						return nil
					},
				},
			},
			// if one element of the list fails validation
			// the entire list is removed. Terraform does not return
			// list indexes as part of the diagnostic attribute path
			expectedProps: map[string]interface{}{},
		},

		// ValidateDiagFunc & ValidateFunc are not supported for TypeList & TypeSet, but they
		// are supported on the nested elements. For now we are not processing the results of those with `schema.Resource` elements
		// since it can get complicated. Nothing will get dropped and the validation error will pass through
		{
			name: "TypeList do not validate nested fields",
			cloudVal: []interface{}{
				map[string]interface{}{
					"nested_prop":       "ABC",
					"nested_other_prop": "ABC",
				},
			},
			schema: schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
								return diag.Errorf("Error")
							},
						},
						"nested_other_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
								return nil
							},
						},
					},
				},
			},
			expectedProps: map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nestedOtherProp": "ABC",
					"nestedProp":      "ABC",
				},
			},
			expectFailure: true,
		},
		{
			name: "TypeSet Do not validate nested fields",
			cloudVal: []interface{}{
				map[string]interface{}{
					"nested_prop": "ABC",
				},
			},
			schema: schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(i interface{}, s string) ([]string, []error) {
								return []string{}, []error{errors.New("Error")}
							},
						},
					},
				},
			},
			expectedProps: map[string]interface{}{
				"collectionProps": []interface{}{
					map[string]interface{}{
						"nestedProp": "ABC",
					},
				},
			},
			expectFailure: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resMap := map[string]*schema.Resource{
				"prov_test": {
					Schema: map[string]*schema.Schema{
						"collection_prop": &tc.schema,
						"other_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
					Importer: &schema.ResourceImporter{
						StateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) ([]*schema.ResourceData, error) {
							err := rd.Set("collection_prop", tc.cloudVal)
							assert.NoError(t, err)
							err = rd.Set("other_prop", "test")
							assert.NoError(t, err)
							return []*schema.ResourceData{rd}, nil
						},
					},
					ReadContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
						err := rd.Set("collection_prop", tc.cloudVal)
						assert.NoError(t, err)
						err = rd.Set("other_prop", "test")
						assert.NoError(t, err)
						return nil
					},
				},
			}
			tfp := &schema.Provider{ResourcesMap: resMap}
			bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
			program := `
name: test
runtime: yaml
`
			pt := pulcheck.PulCheck(t, bridgedProvider, program)
			outPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "out.yaml")

			imp := pt.Import(t, "prov:index/test:Test", "mainRes", "mainRes", "", "--out", outPath)
			tc.expectedProps["otherProp"] = "test"

			contents, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			expected := PulumiYaml{
				Resources: map[string]PulumiResources{
					"mainRes": {
						Type:       "prov:Test",
						Properties: tc.expectedProps,
					},
				},
			}
			var actual PulumiYaml
			err = yaml.Unmarshal(contents, &actual)
			assert.NoError(t, err)

			assert.Equal(t, expected, actual)
			if tc.expectFailure {
				assert.Contains(t, imp.Stdout, "One or more imported inputs failed to validate")
			} else {
				assert.NotContains(t, imp.Stdout, "One or more imported inputs failed to validate")

				f, err := os.OpenFile(filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
				assert.NoError(t, err)
				defer f.Close()
				_, err = f.WriteString(string(contents))
				assert.NoError(t, err)

				// run preview using the generated file
				pt.Preview(t, optpreview.Diff(), optpreview.ExpectNoChanges())
			}
		})
	}
}
