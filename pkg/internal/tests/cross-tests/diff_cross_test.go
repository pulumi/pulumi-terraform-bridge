// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and

// Various test cases for comparing bridged provider behavior against the equivalent TF provider.
package crosstests

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestUnchangedBasicObject(t *testing.T) {
	t.Parallel()
	cfg := map[string]any{"f0": []any{map[string]any{"x": "ok"}}}
	runDiffCheck(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"f0": {
					Required: true,
					Type:     schema.TypeList,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"x": {Optional: true, Type: schema.TypeString},
						},
					},
				},
			},
		},
		Config1: cfg,
		Config2: cfg,
	})
}

func TestDiffBasicTypes(t *testing.T) {
	t.Parallel()

	typeCases := []struct {
		name             string
		config1, config2 any
		prop             *schema.Schema
	}{
		{
			name:    "string",
			config1: map[string]any{"prop": "A"},
			config2: map[string]any{"prop": "B"},
			prop: &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
		{
			name:    "int",
			config1: map[string]any{"prop": 1},
			config2: map[string]any{"prop": 2},
			prop: &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
		},
		{
			name:    "float",
			config1: map[string]any{"prop": 1.1},
			config2: map[string]any{"prop": 2.2},
			prop: &schema.Schema{
				Type:     schema.TypeFloat,
				Optional: true,
			},
		},
		{
			name:    "bool",
			config1: map[string]any{"prop": true},
			config2: map[string]any{"prop": false},
			prop: &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
		},
		{
			name:    "list attr",
			config1: map[string]any{"prop": []any{"A", "B"}},
			config2: map[string]any{"prop": []any{"A", "C"}},
			prop: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		{
			name:    "set attr",
			config1: map[string]any{"prop": []any{"A", "B"}},
			config2: map[string]any{"prop": []any{"A", "C"}},
			prop: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		{
			name:    "map attr",
			config1: map[string]any{"prop": map[string]any{"A": "B"}},
			config2: map[string]any{"prop": map[string]any{"A": "C"}},
			prop: &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		{
			name: "list block",
			config1: map[string]any{
				"prop": []any{map[string]any{"x": "A"}, map[string]any{"x": "B"}},
			},
			config2: map[string]any{
				"prop": []any{map[string]any{"x": "A"}, map[string]any{"x": "C"}},
			},
			prop: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {Optional: true, Type: schema.TypeString},
					},
				},
			},
		},
		{
			name: "set block",
			config1: map[string]any{
				"prop": []any{map[string]any{"x": "A"}, map[string]any{"x": "B"}},
			},
			config2: map[string]any{
				"prop": []any{map[string]any{"x": "A"}, map[string]any{"x": "C"}},
			},
			prop: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {Optional: true, Type: schema.TypeString},
					},
				},
			},
		},
	}

	for _, tc := range typeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := func(forceNew bool) *schema.Resource {
				res := &schema.Resource{
					Schema: map[string]*schema.Schema{
						"other_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"prop": tc.prop,
					},
				}

				if forceNew {
					res.Schema["prop"].ForceNew = true
					if nestedRes, ok := res.Schema["prop"].Elem.(*schema.Resource); ok {
						nestedRes.Schema["x"].ForceNew = true
					}
				}

				return res
			}

			t.Run("no diff", func(t *testing.T) {
				tfAction := runDiffCheck(t, diffTestCase{
					Resource: res(false),
					Config1:  tc.config1,
					Config2:  tc.config1,
				})

				require.Equal(t, []string{"no-op"}, tfAction.TFDiff.Actions)
			})

			t.Run("diff", func(t *testing.T) {
				tfAction := runDiffCheck(t, diffTestCase{
					Resource: res(false),
					Config1:  tc.config1,
					Config2:  tc.config2,
				})

				require.Equal(t, []string{"update"}, tfAction.TFDiff.Actions)
			})

			t.Run("create", func(t *testing.T) {
				tfAction := runDiffCheck(t, diffTestCase{
					Resource: res(false),
					Config1:  nil,
					Config2:  tc.config1,
				})

				require.Equal(t, []string{"create"}, tfAction.TFDiff.Actions)
			})

			t.Run("delete", func(t *testing.T) {
				tfAction := runDiffCheck(t, diffTestCase{
					Resource: res(false),
					Config1:  tc.config1,
					Config2:  nil,
				})

				require.Equal(t, []string{"delete"}, tfAction.TFDiff.Actions)
			})

			t.Run("replace", func(t *testing.T) {
				tfAction := runDiffCheck(t, diffTestCase{
					Resource: res(true),
					Config1:  tc.config1,
					Config2:  tc.config2,
				})

				require.Equal(t, []string{"create", "delete"}, tfAction.TFDiff.Actions)
			})

			t.Run("replace delete first", func(t *testing.T) {
				tfAction := runDiffCheck(t, diffTestCase{
					Resource:            res(true),
					Config1:             tc.config1,
					Config2:             tc.config2,
					DeleteBeforeReplace: true,
				})

				require.Equal(t, []string{"delete", "create"}, tfAction.TFDiff.Actions)
			})
		})
	}
}

func TestSetReordering(t *testing.T) {
	t.Parallel()
	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"set": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1: map[string]any{
			"set": []any{"A", "B"},
		},
		Config2: map[string]any{
			"set": []any{"B", "A"},
		},
	})
}

// If a list-nested block has Required set, it cannot be empty. TF emits an error. Pulumi currently panics.
//
//	│ Error: Insufficient f0 blocks
//	│
//	│   on test.tf line 1, in resource "crossprovider_testres" "example":
//	│    1: resource "crossprovider_testres" "example" {
func TestEmptyRequiredList(t *testing.T) {
	t.Parallel()
	t.Skip("TODO - fix panic and make a negative test here")
	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"f0": {
						Type:      schema.TypeString,
						Required:  true,
						Sensitive: true,
					},
				}},
			},
		},
	}

	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1:  map[string]any{"f0": []any{}},
		Config2:  map[string]any{"f0": []any{}},
	})
}

func TestAws2442(t *testing.T) {
	t.Parallel()
	hashes := map[int]string{}

	stringHashcode := func(s string) int {
		v := int(crc32.ChecksumIEEE([]byte(s)))
		if v >= 0 {
			return v
		}
		if -v >= 0 {
			return -v
		}
		// v == MinInt
		return 0
	}

	resourceParameterHash := func(v interface{}) int {
		var buf bytes.Buffer
		m := v.(map[string]interface{})
		// Store the value as a lower case string, to match how we store them in FlattenParameters
		name := strings.ToLower(m["name"].(string))
		buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(m["name"].(string))))
		buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(m["apply_method"].(string))))
		buf.WriteString(fmt.Sprintf("%s-", m["value"].(string)))

		// This hash randomly affects the "order" of the set, which affects in what order parameters
		// are applied, when there are more than 20 (chunked).
		n := stringHashcode(buf.String())

		if old, ok := hashes[n]; ok {
			if old != name {
				panic("Hash collision on " + name)
			}
		}
		hashes[n] = name
		return n
	}

	rschema := map[string]*schema.Schema{
		"parameter": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"apply_method": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "immediate",
					},
					"name": {
						Type:     schema.TypeString,
						Required: true,
					},
					"value": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
			Set: resourceParameterHash,
		},
	}
	resource := &schema.Resource{
		Schema: rschema,
		CreateContext: func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			rd.SetId("someid") // CreateContext must pick an ID
			parameterList := rd.Get("parameter").(*schema.Set).List()
			slices.Reverse(parameterList)
			// Now intentionally reorder parameters away from the canonical order.
			err := rd.Set("parameter", parameterList[0:3])
			require.NoError(t, err)
			return make(diag.Diagnostics, 0)
		},
		// UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
		// 	panic("UPD")
		// },
	}

	type parameter struct {
		name        string
		value       string
		applyMethod string
	}

	parameters := []parameter{
		{
			name:        "max_connections",
			value:       "500",
			applyMethod: "pending-reboot",
		},
		{
			name:        "wal_buffers",
			value:       "2048",
			applyMethod: "pending-reboot",
		}, // in 8kB
		{
			name:        "default_statistics_target",
			value:       "100",
			applyMethod: "immediate",
		},
		{
			name:        "random_page_cost",
			value:       "1.1",
			applyMethod: "immediate",
		},
		{
			name:        "effective_io_concurrency",
			value:       "200",
			applyMethod: "immediate",
		},
		{
			name:        "work_mem",
			value:       "65536",
			applyMethod: "immediate",
		}, // in kB
		{
			name:        "max_parallel_workers_per_gather",
			value:       "4",
			applyMethod: "immediate",
		},
		{
			name:        "max_parallel_maintenance_workers",
			value:       "4",
			applyMethod: "immediate",
		},
		{
			name:        "pg_stat_statements.track",
			value:       "ALL",
			applyMethod: "immediate",
		},
		{
			name:        "shared_preload_libraries",
			value:       "pg_stat_statements,auto_explain",
			applyMethod: "pending-reboot",
		},
		{
			name:        "track_io_timing",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_min_duration_statement",
			value:       "1000",
			applyMethod: "immediate",
		},
		{
			name:        "log_lock_waits",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_temp_files",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_checkpoints",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_connections",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_disconnections",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_autovacuum_min_duration",
			value:       "0",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_format",
			value:       "json",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_min_duration",
			value:       "1000",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_analyze",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_buffers",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_timing",
			value:       "0",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_triggers",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_verbose",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_nested_statements",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.sample_rate",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "rds.logical_replication",
			value:       "1",
			applyMethod: "pending-reboot",
		},
	}

	jsonifyParameters := func(parameters []parameter) []interface{} {
		var result []interface{}
		for _, p := range parameters {
			result = append(result, map[string]interface{}{
				"name":         p.name,
				"value":        p.value,
				"apply_method": p.applyMethod,
			})
		}
		return result
	}

	cfg := map[string]any{
		"parameter": jsonifyParameters(parameters),
	}

	ps := jsonifyParameters(parameters)
	slices.Reverse(ps)
	cfg2 := map[string]any{
		"parameter": ps,
	}

	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1:  cfg,
		Config2:  cfg2,
	})
}

func TestSimpleOptionalComputed(t *testing.T) {
	t.Parallel()
	emptyConfig := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
	nonEmptyConfig := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"name": tftypes.String,
			},
		},
		map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, "A")},
	)
	for _, tc := range []struct {
		name    string
		config1 tftypes.Value
		config2 tftypes.Value
	}{
		{"empty to empty", emptyConfig, emptyConfig},
		{"empty to non-empty", emptyConfig, nonEmptyConfig},
		{"non-empty to empty", nonEmptyConfig, emptyConfig},
		{"non-empty to non-empty", nonEmptyConfig, nonEmptyConfig},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runDiffCheck(t, diffTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
					CreateContext: func(
						ctx context.Context, rd *schema.ResourceData, i interface{},
					) diag.Diagnostics {
						err := rd.Set("name", "ComputedVal")
						require.NoError(t, err)
						rd.SetId("someid")
						return make(diag.Diagnostics, 0)
					},
				},
				Config1: tc.config1,
				Config2: tc.config2,
			})
		})
	}
}

func TestOptionalComputedAttrCollection(t *testing.T) {
	t.Parallel()
	emptyConfig := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
	t0 := tftypes.List{ElementType: tftypes.String}
	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"security_groups": t0,
		},
	}
	nonEmptyConfig := tftypes.NewValue(
		t1,
		map[string]tftypes.Value{
			"security_groups": tftypes.NewValue(
				t0, []tftypes.Value{tftypes.NewValue(tftypes.String, "sg1")},
			),
		},
	)
	for _, tc := range []struct {
		name     string
		maxItems int
		typ      schema.ValueType
		config1  tftypes.Value
		config2  tftypes.Value
	}{
		{"list empty to empty", 0, schema.TypeList, emptyConfig, emptyConfig},
		{"list empty to non-empty", 0, schema.TypeList, emptyConfig, nonEmptyConfig},
		{"list non-empty to empty", 0, schema.TypeList, nonEmptyConfig, emptyConfig},
		{"list non-empty to non-empty", 0, schema.TypeList, nonEmptyConfig, nonEmptyConfig},
		{"set empty to empty", 0, schema.TypeSet, emptyConfig, emptyConfig},
		{"set empty to non-empty", 0, schema.TypeSet, emptyConfig, nonEmptyConfig},
		{"set non-empty to empty", 0, schema.TypeSet, nonEmptyConfig, emptyConfig},
		{"set non-empty to non-empty", 0, schema.TypeSet, nonEmptyConfig, nonEmptyConfig},
		{"list max items one empty to empty", 1, schema.TypeList, emptyConfig, emptyConfig},
		{"list max items one empty to non-empty", 1, schema.TypeList, emptyConfig, nonEmptyConfig},
		{"list max items one non-empty to empty", 1, schema.TypeList, nonEmptyConfig, emptyConfig},
		{"list max items one non-empty to non-empty", 1, schema.TypeList, nonEmptyConfig, nonEmptyConfig},
		{"set max items one empty to empty", 1, schema.TypeSet, emptyConfig, emptyConfig},
		{"set max items one empty to non-empty", 1, schema.TypeSet, emptyConfig, nonEmptyConfig},
		{"set max items one non-empty to empty", 1, schema.TypeSet, nonEmptyConfig, emptyConfig},
		{"set max items one non-empty to non-empty", 1, schema.TypeSet, nonEmptyConfig, nonEmptyConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runDiffCheck(t, diffTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"security_groups": {
							Type:     tc.typ,
							Optional: true,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
							MaxItems: tc.maxItems,
						},
					},
					CreateContext: func(
						ctx context.Context, rd *schema.ResourceData, i interface{},
					) diag.Diagnostics {
						err := rd.Set("security_groups", []string{"ComputedSG"})
						require.NoError(t, err)
						rd.SetId("someid")
						return make(diag.Diagnostics, 0)
					},
				},
				Config1: tc.config1,
				Config2: tc.config2,
			})
		})
	}
}

func TestOptionalComputedBlockCollection(t *testing.T) {
	t.Parallel()
	emptyConfig := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
	t0 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"foo": tftypes.String,
		},
	}
	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"security_groups": tftypes.List{ElementType: t0},
		},
	}

	nonEmptyConfig := tftypes.NewValue(t1, map[string]tftypes.Value{
		"security_groups": tftypes.NewValue(tftypes.List{ElementType: t0}, []tftypes.Value{
			tftypes.NewValue(t0,
				map[string]tftypes.Value{"foo": tftypes.NewValue(tftypes.String, "sg1")}),
		}),
	})

	for _, tc := range []struct {
		name     string
		maxItems int
		typ      schema.ValueType
		config1  tftypes.Value
		config2  tftypes.Value
	}{
		{"list empty to empty", 0, schema.TypeList, emptyConfig, emptyConfig},
		{"list empty to non-empty", 0, schema.TypeList, emptyConfig, nonEmptyConfig},
		{"list non-empty to empty", 0, schema.TypeList, nonEmptyConfig, emptyConfig},
		{"list non-empty to non-empty", 0, schema.TypeList, nonEmptyConfig, nonEmptyConfig},
		{"set empty to empty", 0, schema.TypeSet, emptyConfig, emptyConfig},
		{"set empty to non-empty", 0, schema.TypeSet, emptyConfig, nonEmptyConfig},
		{"set non-empty to empty", 0, schema.TypeSet, nonEmptyConfig, emptyConfig},
		{"set non-empty to non-empty", 0, schema.TypeSet, nonEmptyConfig, nonEmptyConfig},
		{"list max items one empty to empty", 1, schema.TypeList, emptyConfig, emptyConfig},
		{"list max items one empty to non-empty", 1, schema.TypeList, emptyConfig, nonEmptyConfig},
		{"list max items one non-empty to empty", 1, schema.TypeList, nonEmptyConfig, emptyConfig},
		{"list max items one non-empty to non-empty", 1, schema.TypeList, nonEmptyConfig, nonEmptyConfig},
		{"set max items one empty to empty", 1, schema.TypeSet, emptyConfig, emptyConfig},
		{"set max items one empty to non-empty", 1, schema.TypeSet, emptyConfig, nonEmptyConfig},
		{"set max items one non-empty to empty", 1, schema.TypeSet, nonEmptyConfig, emptyConfig},
		{"set max items one non-empty to non-empty", 1, schema.TypeSet, nonEmptyConfig, nonEmptyConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runDiffCheck(t, diffTestCase{
				Resource: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"security_groups": {
							Type:     tc.typ,
							Optional: true,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"foo": {
										Optional: true,
										Type:     schema.TypeString,
									},
								},
							},
							MaxItems: tc.maxItems,
						},
					},
					CreateContext: func(
						ctx context.Context, rd *schema.ResourceData, i interface{},
					) diag.Diagnostics {
						err := rd.Set("security_groups", []any{map[string]any{"foo": "ComputedSG"}})
						require.NoError(t, err)
						rd.SetId("someid")
						return make(diag.Diagnostics, 0)
					},
				},
				Config1: tc.config1,
				Config2: tc.config2,
			})
		})
	}
}

func TestComputedSetFieldsNoDiff(t *testing.T) {
	t.Parallel()
	elemSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"metro_code": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"metro_name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
	}

	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"location": {
				Required: true,
				MaxItems: 1,
				Type:     schema.TypeSet,
				Elem:     &elemSchema,
			},
		},
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("r1")
			// The field is computed and the provider always returns a metro_name.
			err := rd.Set("location", schema.NewSet(schema.HashResource(&elemSchema), []interface{}{
				map[string]interface{}{"metro_name": "Frankfurt", "metro_code": "FR"},
			}))
			require.NoError(t, err)
			return diag.Diagnostics{}
		},
	}

	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"metro_code": tftypes.String,
		},
	}

	t0 := tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"location": tftypes.Set{
					ElementType: t1,
				},
			},
		},
		map[string]tftypes.Value{
			"location": tftypes.NewValue(
				tftypes.Set{ElementType: t1},
				[]tftypes.Value{
					tftypes.NewValue(t1, map[string]tftypes.Value{
						// We try to set the metro_code but the provider should return metro_name
						"metro_code": tftypes.NewValue(tftypes.String, "FR"),
					}),
				},
			),
		},
	)
	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1:  t0,
		Config2:  t0,
	})
}

func TestMaxItemsOneCollectionOnlyDiff(t *testing.T) {
	t.Parallel()
	sch := map[string]*schema.Schema{
		"rule": {
			Type:     schema.TypeList,
			Required: true,
			MaxItems: 1000,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"filter": {
						Type:     schema.TypeList,
						Optional: true,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"prefix": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
					},
				},
			},
		},
	}

	t1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"prefix": tftypes.String,
		},
	}

	t2 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"filter": tftypes.List{ElementType: t1},
		},
	}

	t3 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"rule": tftypes.List{ElementType: t2},
		},
	}

	v1 := tftypes.NewValue(
		t3,
		map[string]tftypes.Value{
			"rule": tftypes.NewValue(
				tftypes.List{ElementType: t2},
				[]tftypes.Value{
					tftypes.NewValue(
						t2,
						map[string]tftypes.Value{
							"filter": tftypes.NewValue(
								tftypes.List{ElementType: t1},
								[]tftypes.Value{},
							),
						},
					),
				},
			),
		},
	)

	v2 := tftypes.NewValue(
		t3,
		map[string]tftypes.Value{
			"rule": tftypes.NewValue(
				tftypes.List{ElementType: t2},
				[]tftypes.Value{
					tftypes.NewValue(
						t2,
						map[string]tftypes.Value{
							"filter": tftypes.NewValue(
								tftypes.List{ElementType: t1},
								[]tftypes.Value{
									tftypes.NewValue(
										t1,
										map[string]tftypes.Value{
											"prefix": tftypes.NewValue(tftypes.String, nil),
										},
									),
								},
							),
						},
					),
				},
			),
		},
	)

	diff := runDiffCheck(
		t,
		diffTestCase{
			Resource: &schema.Resource{Schema: sch},
			Config1:  v1,
			Config2:  v2,
		},
	)

	getFilter := func(val map[string]any) any {
		return val["rule"].([]any)[0].(map[string]any)["filter"]
	}

	t.Log(diff.PulumiDiff)
	require.Equal(t, []string{"update"}, diff.TFDiff.Actions)
	require.NotEqual(t, getFilter(diff.TFDiff.Before), getFilter(diff.TFDiff.After))
	require.True(t, findKeyInPulumiDetailedDiff(diff.PulumiDiff.DetailedDiff, "rules[0].filter"))
}

func TestNilVsEmptyListProperty(t *testing.T) {
	t.Parallel()
	cfgEmpty := map[string]any{"f0": []any{}}
	cfgNil := map[string]any{}

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}

	t.Run("nil to empty", func(t *testing.T) {
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  cfgNil,
			Config2:  cfgEmpty,
		})

		require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
	})

	t.Run("empty to nil", func(t *testing.T) {
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  cfgEmpty,
			Config2:  cfgNil,
		})

		require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
	})
}

func TestNilVsEmptyMapProperty(t *testing.T) {
	t.Parallel()
	cfgEmpty := map[string]any{"f0": map[string]any{}}
	cfgNil := map[string]any{}

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}

	t.Run("nil to empty", func(t *testing.T) {
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  cfgNil,
			Config2:  cfgEmpty,
		})

		require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
	})

	t.Run("empty to nil", func(t *testing.T) {
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  cfgEmpty,
			Config2:  cfgNil,
		})

		require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
	})
}

func findKindInPulumiDetailedDiff(detailedDiff map[string]interface{}, key string) bool {
	for _, val := range detailedDiff {
		// ADD is a valid kind but is the default value for kind
		// This means that it is missed out from the representation
		if key == "ADD" {
			if len(val.(map[string]interface{})) == 0 {
				return true
			}
		}
		if val.(map[string]interface{})["kind"] == key {
			return true
		}
	}
	return false
}

func findKeyInPulumiDetailedDiff(detailedDiff map[string]interface{}, key string) bool {
	for k := range detailedDiff {
		if k == key {
			return true
		}
	}
	return false
}

func TestNilVsEmptyNestedCollections(t *testing.T) {
	t.Parallel()
	for _, MaxItems := range []int{0, 1} {
		t.Run(fmt.Sprintf("MaxItems=%d", MaxItems), func(t *testing.T) {
			res := &schema.Resource{
				Schema: map[string]*schema.Schema{
					"list": {
						Type:     schema.TypeList,
						Optional: true,
						MaxItems: MaxItems,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"x": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Schema{
										Type: schema.TypeString,
									},
								},
							},
						},
					},
					"set": {
						Type:     schema.TypeSet,
						Optional: true,
						MaxItems: MaxItems,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"x": {
									Type:     schema.TypeList,
									Optional: true,
									Elem: &schema.Schema{
										Type: schema.TypeString,
									},
								},
							},
						},
					},
				},
			}

			t.Run("nil to empty list", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  map[string]any{},
					Config2:  map[string]any{"list": []any{}},
				})

				require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
			})

			t.Run("nil to empty set", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  map[string]any{},
					Config2:  map[string]any{"set": []any{}},
				})
				require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
			})

			t.Run("empty to nil list", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  map[string]any{"list": []any{}},
					Config2:  map[string]any{},
				})
				require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
			})

			t.Run("empty to nil set", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  map[string]any{"set": []any{}},
					Config2:  map[string]any{},
				})
				require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
			})

			listOfStrType := tftypes.List{ElementType: tftypes.String}

			objType := tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": listOfStrType,
				},
			}

			listType := tftypes.List{ElementType: objType}

			listVal := tftypes.NewValue(
				listType,
				[]tftypes.Value{
					tftypes.NewValue(
						objType,
						map[string]tftypes.Value{
							"x": tftypes.NewValue(listOfStrType,
								[]tftypes.Value{}),
						},
					),
				},
			)

			listConfig := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"list": listType,
					},
				},
				map[string]tftypes.Value{
					"list": listVal,
				},
			)

			t.Run("nil to empty list in list", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  map[string]any{},
					Config2:  listConfig,
				})

				require.Equal(t, []string{"update"}, diff.TFDiff.Actions)
				require.NotEqual(t, diff.TFDiff.Before, diff.TFDiff.After)
				require.True(t, findKindInPulumiDetailedDiff(diff.PulumiDiff.DetailedDiff, "ADD"))
			})

			t.Run("empty list in list to nil", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  listConfig,
					Config2:  map[string]any{},
				})

				require.Equal(t, []string{"update"}, diff.TFDiff.Actions)
				require.NotEqual(t, diff.TFDiff.Before, diff.TFDiff.After)
				require.True(t, findKindInPulumiDetailedDiff(diff.PulumiDiff.DetailedDiff, "DELETE"))
			})

			setType := tftypes.Set{ElementType: objType}

			setVal := tftypes.NewValue(
				setType,
				[]tftypes.Value{
					tftypes.NewValue(
						objType,
						map[string]tftypes.Value{
							"x": tftypes.NewValue(listOfStrType,
								[]tftypes.Value{}),
						},
					),
				},
			)

			setConfig := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"set": setType,
					},
				},
				map[string]tftypes.Value{
					"set": setVal,
				},
			)

			t.Run("nil to empty list in set", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  map[string]any{},
					Config2:  setConfig,
				})

				require.Equal(t, []string{"update"}, diff.TFDiff.Actions)
				require.NotEqual(t, diff.TFDiff.Before, diff.TFDiff.After)
				t.Log(diff.PulumiDiff.DetailedDiff)
				require.True(t, findKindInPulumiDetailedDiff(diff.PulumiDiff.DetailedDiff, "ADD"))
			})

			t.Run("empty list in set to nil", func(t *testing.T) {
				diff := runDiffCheck(t, diffTestCase{
					Resource: res,
					Config1:  setConfig,
					Config2:  map[string]any{},
				})

				require.Equal(t, []string{"no-op"}, diff.TFDiff.Actions)
			})
		})
	}
}

func TestAttributeCollectionForceNew(t *testing.T) {
	t.Parallel()
	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"list": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"set": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"map": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}

	t.Run("list", func(t *testing.T) {
		t.Run("changed non-empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{"A"}},
				Config2:  map[string]any{"list": []any{"B"}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE_REPLACE"))
		})

		t.Run("changed to empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{"A"}},
				Config2:  map[string]any{"list": []any{}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "DELETE_REPLACE"))
		})

		t.Run("changed from empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{}},
				Config2:  map[string]any{"list": []any{"A"}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "ADD_REPLACE"))
		})
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()
		t.Run("changed non-empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{"A"}},
				Config2:  map[string]any{"set": []any{"B"}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE_REPLACE"))
		})

		t.Run("changed to empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{"A"}},
				Config2:  map[string]any{"set": []any{}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "DELETE_REPLACE"))
		})

		t.Run("changed from empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{}},
				Config2:  map[string]any{"set": []any{"A"}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "ADD_REPLACE"))
		})
	})

	t.Run("map", func(t *testing.T) {
		t.Parallel()
		t.Run("changed non-empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"map": map[string]any{"A": "A"}},
				Config2:  map[string]any{"map": map[string]any{"A": "B"}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE_REPLACE"))
		})

		t.Run("changed to empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"map": map[string]any{"A": "A"}},
				Config2:  map[string]any{"map": map[string]any{}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "DELETE_REPLACE"))
		})

		t.Run("changed from empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"map": map[string]any{}},
				Config2:  map[string]any{"map": map[string]any{"A": "A"}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "ADD_REPLACE"))
		})
	})
}

func TestBlockCollectionForceNew(t *testing.T) {
	t.Parallel()
	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"list": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"set": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"other": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	t.Run("list", func(t *testing.T) {
		t.Parallel()
		t.Run("changed non-empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"list": []any{map[string]any{"x": "B"}}},
			})

			require.Equal(t, []string{"update"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE"))
			require.False(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE_REPLACE"))
		})

		t.Run("changed to empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"list": []any{}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "DELETE_REPLACE"))
		})

		t.Run("changed from empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{}},
				Config2:  map[string]any{"list": []any{map[string]any{"x": "A"}}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "ADD_REPLACE"))
		})
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()
		t.Run("changed non-empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"set": []any{map[string]any{"x": "B"}}},
			})

			require.Equal(t, []string{"update"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE"))
			require.False(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE_REPLACE"))
		})

		t.Run("changed to empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"set": []any{}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "DELETE_REPLACE"))
		})

		t.Run("changed from empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{}},
				Config2:  map[string]any{"set": []any{map[string]any{"x": "A"}}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "ADD_REPLACE"))
		})
	})
}

func TestBlockCollectionElementForceNew(t *testing.T) {
	t.Parallel()
	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"list": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
			"set": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
		},
	}

	t.Run("list", func(t *testing.T) {
		t.Parallel()
		t.Run("changed non-empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"list": []any{map[string]any{"x": "B"}}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE_REPLACE"))
		})

		t.Run("changed to empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"list": []any{}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "DELETE_REPLACE"))
		})

		t.Run("changed from empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"list": []any{}},
				Config2:  map[string]any{"list": []any{map[string]any{"x": "A"}}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "ADD_REPLACE"))
		})
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()
		t.Run("changed non-empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"set": []any{map[string]any{"x": "B"}}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "UPDATE_REPLACE"))
		})

		t.Run("changed to empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{map[string]any{"x": "A"}}},
				Config2:  map[string]any{"set": []any{}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "DELETE_REPLACE"))
		})

		t.Run("changed from empty", func(t *testing.T) {
			t.Parallel()
			res := runDiffCheck(t, diffTestCase{
				Resource: res,
				Config1:  map[string]any{"set": []any{}},
				Config2:  map[string]any{"set": []any{map[string]any{"x": "A"}}},
			})

			require.Equal(t, []string{"create", "delete"}, res.TFDiff.Actions)
			require.True(t, findKindInPulumiDetailedDiff(res.PulumiDiff.DetailedDiff, "ADD_REPLACE"))
		})
	})
}

func TestDetailedDiffReplacementComputedProperty(t *testing.T) {
	t.Parallel()
	// TODO[pulumi/pulumi-terraform-bridge#2660]
	// We fail to re-compute computed properties when the resource is being replaced.
	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"computed": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"other": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, meta interface{}) diag.Diagnostics {
			rd.SetId("r1")

			err := rd.Set("computed", "computed_value")
			contract.AssertNoErrorf(err, "setting computed")
			return nil
		},
	}

	type testOutput struct {
		initialValue cty.Value
		changeValue  cty.Value
		tfOut        string
		pulumiOut    string
		detailedDiff map[string]any
	}

	t.Run("no change", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.ObjectVal(map[string]cty.Value{})
		changeValue := cty.ObjectVal(map[string]cty.Value{})
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  initialValue,
			Config2:  changeValue,
		})

		autogold.ExpectFile(t, testOutput{
			initialValue: initialValue,
			changeValue:  changeValue,
			tfOut:        diff.TFOut,
			pulumiOut:    diff.PulumiOut,
			detailedDiff: diff.PulumiDiff.DetailedDiff,
		})
	})

	t.Run("non-computed added", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.ObjectVal(map[string]cty.Value{})
		changeValue := cty.ObjectVal(map[string]cty.Value{"other": cty.StringVal("other_value")})
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  initialValue,
			Config2:  changeValue,
		})

		autogold.ExpectFile(t, testOutput{
			initialValue: initialValue,
			changeValue:  changeValue,
			tfOut:        diff.TFOut,
			pulumiOut:    diff.PulumiOut,
			detailedDiff: diff.PulumiDiff.DetailedDiff,
		})
	})

	t.Run("non-computed removed", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.ObjectVal(map[string]cty.Value{"other": cty.StringVal("other_value")})
		changeValue := cty.ObjectVal(map[string]cty.Value{})
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  initialValue,
			Config2:  changeValue,
		})

		autogold.ExpectFile(t, testOutput{
			initialValue: initialValue,
			changeValue:  changeValue,
			tfOut:        diff.TFOut,
			pulumiOut:    diff.PulumiOut,
			detailedDiff: diff.PulumiDiff.DetailedDiff,
		})
	})

	t.Run("non-computed changed", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.ObjectVal(map[string]cty.Value{"other": cty.StringVal("other_value")})
		changeValue := cty.ObjectVal(map[string]cty.Value{"other": cty.StringVal("other_value_2")})
		diff := runDiffCheck(t, diffTestCase{
			Resource: res,
			Config1:  initialValue,
			Config2:  changeValue,
		})

		autogold.ExpectFile(t, testOutput{
			initialValue: initialValue,
			changeValue:  changeValue,
			tfOut:        diff.TFOut,
			pulumiOut:    diff.PulumiOut,
			detailedDiff: diff.PulumiDiff.DetailedDiff,
		})
	})
}
