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
	"github.com/stretchr/testify/require"
)

func TestUnchangedBasicObject(t *testing.T) {
	skipUnlessLinux(t)
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

func TestSimpleStringNoChange(t *testing.T) {
	skipUnlessLinux(t)
	config := map[string]any{"name": "A"}
	runDiffCheck(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		Config1: config,
		Config2: config,
	})
}

func TestSimpleStringRename(t *testing.T) {
	skipUnlessLinux(t)
	runDiffCheck(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		Config1: map[string]any{
			"name": "A",
		},
		Config2: map[string]any{
			"name": "B",
		},
	})
}

func TestSetReordering(t *testing.T) {
	skipUnlessLinux(t)
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
	t.Skip("TODO - fix panic and make a negative test here")
	skipUnlessLinux(t)
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
	skipUnlessLinux(t)
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
	skipUnlessLinux(t)
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
		t.Run(tc.name, func(t *testing.T) {
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
	skipUnlessLinux(t)
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
	skipUnlessLinux(t)
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
	skipUnlessLinux(t)

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

func TestTrulyComputedEmptyToNil(t *testing.T) {
	skipUnlessLinux(t)

	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"truly_computed_tags_all": {
				Type:     schema.TypeMap,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"other": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("r1")
			err := rd.Set("truly_computed_tags_all", map[string]interface{}{})
			require.NoError(t, err)
			return diag.Diagnostics{}
		},
		UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			err := rd.Set("truly_computed_tags_all", nil)
			require.NoError(t, err)
			return diag.Diagnostics{}
		},
	}

	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1:  map[string]any{"other": "A"},
		Config2:  map[string]any{"other": "B"},
	})

	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1:  map[string]any{"other": "A"},
		Config2:  map[string]any{"other": "A"},
	})
}
