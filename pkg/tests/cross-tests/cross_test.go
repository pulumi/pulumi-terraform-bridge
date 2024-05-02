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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	webaclschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/internal/webaclschema"
	"github.com/stretchr/testify/assert"
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
					Type:     schema.TypeString,
					Optional: true,
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
		//fmt.Println("setting hash name", n, name)
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

func TestHash(t *testing.T) {
	expected := map[string]interface{}{
		"action": []interface{}{map[string]interface{}{
			"allow":     []interface{}{},
			"block":     []interface{}{interface{}(nil)},
			"captcha":   []interface{}{},
			"challenge": []interface{}{},
			"count":     []interface{}{}}},
		"captcha_config":  []interface{}{},
		"name":            "IPAllowRule",
		"override_action": []interface{}{},
		"priority":        0,
		//"rule_label":      schema.NewSet(nil, nil),
		"statement": []interface{}{map[string]interface{}{
			"and_statement":        []interface{}{},
			"byte_match_statement": []interface{}{},
			"geo_match_statement":  []interface{}{},
			"ip_set_reference_statement": []interface{}{map[string]interface{}{
				"arn":                        "some-arn",
				"ip_set_forwarded_ip_config": []interface{}{},
			}},
			"label_match_statement":                 []interface{}{},
			"managed_rule_group_statement":          []interface{}{},
			"not_statement":                         []interface{}{},
			"or_statement":                          []interface{}{},
			"rate_based_statement":                  []interface{}{},
			"regex_match_statement":                 []interface{}{},
			"regex_pattern_set_reference_statement": []interface{}{},
			"rule_group_reference_statement":        []interface{}{},
			"size_constraint_statement":             []interface{}{},
			"sqli_match_statement":                  []interface{}{},
			"xss_match_statement":                   []interface{}{}}},
		"visibility_config": []interface{}{map[string]interface{}{
			"cloudwatch_metrics_enabled": true,
			"metric_name":                "IPAllowRule",
			"sampled_requests_enabled":   true,
		}}}
	resource := webaclschema.ResourceWebACL()
	resource.Schema = resource.SchemaFunc()
	for i := 0; i < 100; i++ {
		t.Logf("@ %d", i)
		actual := schema.HashResource(resource.Schema["rule"].Elem.(*schema.Resource))(expected)
		assert.Equalf(t, 835885598, actual, "attempt %d", i)
	}
}

func TestAws3880(t *testing.T) {
	cfg := map[string]any{
		"scope": "REGIONAL",
		"name":  "autogenerated-name",
		"default_action": []any{
			map[string]any{
				"allow": []any{map[string]any{}},
			},
		},
		"visibility_config": []any{
			map[string]any{
				"cloudwatch_metrics_enabled": true,
				"metric_name":                "myWebAclMetrics",
				"sampled_requests_enabled":   false,
			},
		},
		"rule": []any{
			map[string]any{
				"action": []any{
					map[string]any{
						"block": []any{map[string]any{}},
					},
				},
				"name":     "IPAllowRule",
				"priority": 0,
				"statement": []any{
					map[string]any{
						"ip_set_reference_statement": []any{map[string]any{
							"arn": "some-arn",
						}},
					},
				},
				"visibility_config": []any{
					map[string]any{
						"cloudwatch_metrics_enabled": true,
						"metric_name":                "IPAllowRule",
						"sampled_requests_enabled":   true,
					},
				},
			},
		},
	}

	resource := webaclschema.ResourceWebACL()
	resource.Schema = resource.SchemaFunc()
	resource.SchemaFunc = nil
	delete(resource.Schema, "tags")
	delete(resource.Schema, "tags_all")

	// Here i may receive maps or slices over base types and *schema.Set which is not friendly to diffing.
	resource.Schema["rule"].Set = func(i interface{}) int {
		actual := schema.HashResource(resource.Schema["rule"].Elem.(*schema.Resource))(i)

		require.NotEqualf(t, 835885598, actual, "This number does not happen under TF")

		im := i.(map[string]interface{})
		ruleLabel := im["rule_label"]
		action := im["action"].([]any)[0].(map[string]interface{})["block"]

		delete(im, "rule_label")

		expected := map[string]interface{}{
			"action": []interface{}{map[string]interface{}{
				"allow":     []interface{}{},
				"block":     []interface{}{interface{}(nil)},
				"captcha":   []interface{}{},
				"challenge": []interface{}{},
				"count":     []interface{}{}}},
			"captcha_config":  []interface{}{},
			"name":            "IPAllowRule",
			"override_action": []interface{}{},
			"priority":        0,
			// "rule_label":      []interface{}{},
			// "rule_label_type": "set",
			"statement": []interface{}{map[string]interface{}{
				"and_statement":        []interface{}{},
				"byte_match_statement": []interface{}{},
				"geo_match_statement":  []interface{}{},
				"ip_set_reference_statement": []interface{}{map[string]interface{}{
					"arn":                        "some-arn",
					"ip_set_forwarded_ip_config": []interface{}{},
				}},
				"label_match_statement":                 []interface{}{},
				"managed_rule_group_statement":          []interface{}{},
				"not_statement":                         []interface{}{},
				"or_statement":                          []interface{}{},
				"rate_based_statement":                  []interface{}{},
				"regex_match_statement":                 []interface{}{},
				"regex_pattern_set_reference_statement": []interface{}{},
				"rule_group_reference_statement":        []interface{}{},
				"size_constraint_statement":             []interface{}{},
				"sqli_match_statement":                  []interface{}{},
				"xss_match_statement":                   []interface{}{}}},
			"visibility_config": []interface{}{map[string]interface{}{
				"cloudwatch_metrics_enabled": true,
				"metric_name":                "IPAllowRule",
				"sampled_requests_enabled":   true,
			}}}

		expected2 := map[string]interface{}{
			"action": []interface{}{map[string]interface{}{
				"allow": []interface{}{},
				"block": []interface{}{
					map[string]any{
						"custom_response": []interface{}{},
					},
				},
				"captcha":   []interface{}{},
				"challenge": []interface{}{},
				"count":     []interface{}{}}},
			"captcha_config":  []interface{}{},
			"name":            "IPAllowRule",
			"override_action": []interface{}{},
			"priority":        0,
			//"rule_label":      schema.NewSet(nil, nil),
			"statement": []interface{}{map[string]interface{}{
				"and_statement":        []interface{}{},
				"byte_match_statement": []interface{}{},
				"geo_match_statement":  []interface{}{},
				"ip_set_reference_statement": []interface{}{map[string]interface{}{
					"arn":                        "some-arn",
					"ip_set_forwarded_ip_config": []interface{}{},
				}},
				"label_match_statement":                 []interface{}{},
				"managed_rule_group_statement":          []interface{}{},
				"not_statement":                         []interface{}{},
				"or_statement":                          []interface{}{},
				"rate_based_statement":                  []interface{}{},
				"regex_match_statement":                 []interface{}{},
				"regex_pattern_set_reference_statement": []interface{}{},
				"rule_group_reference_statement":        []interface{}{},
				"size_constraint_statement":             []interface{}{},
				"sqli_match_statement":                  []interface{}{},
				"xss_match_statement":                   []interface{}{}}},
			"visibility_config": []interface{}{map[string]interface{}{
				"cloudwatch_metrics_enabled": true,
				"metric_name":                "IPAllowRule",
				"sampled_requests_enabled":   true,
			}}}

		switch {
		case assert.ObjectsAreEqual(expected, i):
			fmt.Printf("\n\n#### Computing hash set for rule <<expected>> (action=%#v, ruleLabel=%#v)==> %d\n\n", action, ruleLabel, actual)
		case assert.ObjectsAreEqual(expected2, i):
			fmt.Printf("\n\n#### Computing hash set for rule <<expected2>> (action=%#v, ruleLabel=%#v)==> %d\n\n", action, ruleLabel, actual)
		default:
			assert.Equal(t, expected, i)
		}
		return actual
	}

	runDiffCheck(t, diffTestCase{
		Resource:   resource,
		Config1:    cfg,
		Config2:    cfg,
		SkipPulumi: false,
	})
}

func TestAws3880Minimal(t *testing.T) {
	customResponseSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"custom_response_body_key": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		}
	}
	blockConfigSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"custom_response": customResponseSchema(),
				},
			},
		}
	}
	ruleElement := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"action": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"block": blockConfigSchema(),
					},
				},
			},
		},
	}
	cfg := map[string]any{
		"rule": []any{
			map[string]any{
				"action": []any{
					map[string]any{
						"block": []any{map[string]any{}},
					},
				},
			},
		},
	}

	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"rule": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     ruleElement,
			},
		},
	}

	// Here i may receive maps or slices over base types and *schema.Set which is not friendly to diffing.
	resource.Schema["rule"].Set = func(i interface{}) int {
		actual := schema.HashResource(resource.Schema["rule"].Elem.(*schema.Resource))(i)
		fmt.Printf("hashing %#v as %d\n", i, actual)
		return actual
	}

	runDiffCheck(t, diffTestCase{
		Resource:   resource,
		Config1:    cfg,
		Config2:    cfg,
		SkipPulumi: false,
	})
}
