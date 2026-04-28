// Copyright 2016-2026, Pulumi Corporation.
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
// limitations under the License.

package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// These tests exercise stripStaleDefaults via the real Diff RPC path. The unit tests
// in pkg/tfbridge/strip_stale_defaults_test.go cover the function in isolation, but
// cannot catch issues that arise from the interaction between stripped inputs and
// TF's plan, hash, or detailed-diff machinery.
//
// Two assertion styles are used, calibrated to scenario:
//
//  1. No-op scenarios (unchanged schema, unchanged program): assertNoChanges enforces
//     that the preview reports only "same" — any spurious diff signals a real bug.
//  2. Schema-migration scenarios (provider swapped between Up calls): the strip's
//     effect is verified by inspecting the post-Up exported stack via resourceInputs
//     and asserting the stale field was removed (or, for a changed default, that
//     stored state reflects the new value). Stub TF providers used here do not run
//     validation, so a "no error" assertion would pass even if the strip were
//     deleted — the inputs check is what proves the strip ran.

// assertNoChanges asserts that a preview reports only "same" resources. Use this on
// no-op scenarios (no schema change, program unchanged) to catch spurious diffs that
// would indicate the strip is producing inputs that don't round-trip through TF cleanly.
func assertNoChanges(t *testing.T, summary map[apitype.OpType]int, label string) {
	t.Helper()
	for op, count := range summary {
		require.Equalf(t, apitype.OpSame, op,
			"%s: preview must report only 'same' resources; got %d %q ops (full summary: %+v)",
			label, count, op, summary)
	}
}

// assertOnlySameOrUpdate asserts that a preview reports only "same" or "update"
// resources — never replace, create, or delete. Use this on schema-migration
// scenarios where a transitional in-place update is expected (the strip removes a
// stale value, so the resource updates to drop the field) but a replace would
// indicate the strip caused unintended set-hash or identity changes.
func assertOnlySameOrUpdate(t *testing.T, summary map[apitype.OpType]int, label string) {
	t.Helper()
	for op, count := range summary {
		switch op {
		case apitype.OpSame, apitype.OpUpdate:
			// expected
		default:
			t.Errorf("%s: preview must report only same/update; got %d %q ops (full summary: %+v)",
				label, count, op, summary)
		}
	}
}

// resourceInputs parses an exported stack and returns the stored Pulumi inputs map
// for the first resource whose URN contains urnFragment. Inputs come from Check's
// output and reflect "what the user (after default application) intended."
func resourceInputs(t *testing.T, deployment []byte, urnFragment string) map[string]any {
	t.Helper()
	var stack struct {
		Resources []map[string]any `json:"resources"`
	}
	require.NoError(t, json.Unmarshal(deployment, &stack), "decoding exported stack")
	for _, r := range stack.Resources {
		urn, _ := r["urn"].(string)
		if strings.Contains(urn, urnFragment) {
			val, _ := r["inputs"].(map[string]any)
			require.NotNilf(t, val,
				"resource %q has no stored inputs — the strip's effect cannot be verified", urn)
			return val
		}
	}
	t.Fatalf("no resource matching %q in exported stack", urnFragment)
	return nil
}

// TestStripStaleDefaultsIntegration_DefaultRemoved exercises the original motivating
// scenario: a TF schema default that exists when a resource is created is removed in
// a later provider version. Preview after upgrade must report no spurious diff — the
// strip removes the stale value so PlanResourceChange does not see it.
func TestStripStaleDefaultsIntegration_DefaultRemoved(t *testing.T) {
	t.Parallel()

	withDefault := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"opt_field": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "old-default",
					},
				},
			},
		},
	}
	bp1 := pulcheck.BridgedProvider(t, "prov", withDefault)

	withoutDefault := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"opt_field": {
						Type:     schema.TypeString,
						Optional: true,
						// Default removed.
					},
				},
			},
		},
	}
	bp2 := pulcheck.BridgedProvider(t, "prov", withoutDefault)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
`

	pt := pulcheck.PulCheck(t, bp1, program)
	pt.Up(t)
	stack := pt.ExportStack(t)

	pt2 := pulcheck.PulCheck(t, bp2, program)
	pt2.ImportStack(t, stack)
	preInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	require.Equal(t, "old-default", preInputs["optField"],
		"sanity: imported state should still have the stale value before Up")

	res := pt2.Preview(t, optpreview.Diff())
	assertOnlySameOrUpdate(t, res.ChangeSummary, "DefaultRemoved")
	pt2.Up(t)

	// Strong assertion: the strip removed optField from the new inputs, so the post-Up
	// stored inputs must NOT contain it. Without the strip, Check's "old default"
	// reuse path would re-pin "old-default" into news on every Diff, and the post-Up
	// state would still record optField; this assertion would fail.
	postInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	_, hasField := postInputs["optField"]
	require.False(t, hasField,
		"after Up against the schema with no Default, optField must be absent from stored inputs; got %+v", postInputs)
}

// Note: there is no integration test for the *changed-default* scenario (provider
// upgrade where a TF schema Default goes from v1 to v2). With the strip applied only
// in Diff, the changed-default case is a known phantom-diff limitation — Diff/Preview
// shows v1 → v2 but Update keeps v1 in state. This cannot be fixed by mirroring the
// strip into Update without regressing the legacy-stack falsy-default round-trip
// invariant from PR #3420 (TestUpdatePreservesLegacyFalsyTFDefaults). The TODO near
// stripStaleDefaults points to the architectural cleanup that resolves this.

// TestStripStaleDefaultsIntegration_FieldRemovedFromSchema covers the case where a
// field is removed from the TF schema entirely. The stale value must not be forwarded
// as an unknown attribute to PlanResourceChange.
func TestStripStaleDefaultsIntegration_FieldRemovedFromSchema(t *testing.T) {
	t.Parallel()

	withField := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"keep_field": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"removed_field": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "default-value",
					},
				},
			},
		},
	}
	bp1 := pulcheck.BridgedProvider(t, "prov", withField)

	withoutField := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"keep_field": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
	}
	bp2 := pulcheck.BridgedProvider(t, "prov", withoutField)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      keepField: "value"
`

	pt := pulcheck.PulCheck(t, bp1, program)
	pt.Up(t)
	stack := pt.ExportStack(t)

	pt2 := pulcheck.PulCheck(t, bp2, program)
	pt2.ImportStack(t, stack)
	preInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	require.Equal(t, "default-value", preInputs["removedField"],
		"sanity: imported state should hold the stale removed_field value")

	res := pt2.Preview(t, optpreview.Diff())
	assertOnlySameOrUpdate(t, res.ChangeSummary, "FieldRemovedFromSchema")
	pt2.Up(t)

	// Strong assertion: the strip removed the dropped field entirely; the post-Up
	// stored inputs must not carry it forward.
	postInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	_, hasRemoved := postInputs["removedField"]
	require.Falsef(t, hasRemoved,
		"removedField must be absent from stored inputs after Up; got %+v", postInputs)
	require.Equalf(t, "value", postInputs["keepField"],
		"keepField must be preserved across the upgrade; got %+v", postInputs)
}

// TestStripStaleDefaultsIntegration_NestedTypeListBlock covers the recursion path:
// a TypeList MaxItemsOne block with a nested field whose TF default is removed in
// the upgraded schema. The nested __defaults entry must be stripped without
// disturbing the surrounding block.
func TestStripStaleDefaultsIntegration_NestedTypeListBlock(t *testing.T) {
	t.Parallel()

	withNestedDefault := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"block": {
						Type:     schema.TypeList,
						MaxItems: 1,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"name": {
									Type:     schema.TypeString,
									Optional: true,
								},
								"nested_default": {
									Type:     schema.TypeString,
									Optional: true,
									Default:  "old-nested",
								},
							},
						},
					},
				},
			},
		},
	}
	bp1 := pulcheck.BridgedProvider(t, "prov", withNestedDefault)

	withoutNestedDefault := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"block": {
						Type:     schema.TypeList,
						MaxItems: 1,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"name": {
									Type:     schema.TypeString,
									Optional: true,
								},
								"nested_default": {
									Type:     schema.TypeString,
									Optional: true,
									// Default removed.
								},
							},
						},
					},
				},
			},
		},
	}
	bp2 := pulcheck.BridgedProvider(t, "prov", withoutNestedDefault)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      block:
        name: "alpha"
`

	pt := pulcheck.PulCheck(t, bp1, program)
	pt.Up(t)
	stack := pt.ExportStack(t)

	pt2 := pulcheck.PulCheck(t, bp2, program)
	pt2.ImportStack(t, stack)
	preInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	preBlock, _ := preInputs["block"].(map[string]any)
	require.Equalf(t, "old-nested", preBlock["nestedDefault"],
		"sanity: imported state should hold the stale nested default; got %+v", preBlock)

	res := pt2.Preview(t, optpreview.Diff())
	assertOnlySameOrUpdate(t, res.ChangeSummary, "NestedTypeListBlock")
	pt2.Up(t)

	// Strong assertion: nested recursion stripped the stale nested default.
	postInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	postBlock, ok := postInputs["block"].(map[string]any)
	require.Truef(t, ok, "block must remain in stored inputs; got %+v", postInputs)
	_, hasNested := postBlock["nestedDefault"]
	require.Falsef(t, hasNested,
		"nestedDefault must be stripped from the nested block after Up; got %+v", postBlock)
	require.Equalf(t, "alpha", postBlock["name"],
		"sibling field 'name' must be preserved; got %+v", postBlock)
}

// TestStripStaleDefaultsIntegration_TypeListOfBlocks covers stripArrayOfBlocks: a
// non-MaxItemsOne TypeList of objects, each carrying a nested __defaults entry that
// is removed in the upgraded schema. Distinct from _NestedTypeListBlock which uses
// MaxItems=1 (single object branch). TypeSet is intentionally skipped by the strip
// (set membership is hash-based on element fields), but TypeList elements are
// positional — this test confirms the array recursion path is still exercised for
// TypeList and not accidentally disabled along with TypeSet.
func TestStripStaleDefaultsIntegration_TypeListOfBlocks(t *testing.T) {
	t.Parallel()

	withNested := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"items": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"name": {Type: schema.TypeString, Optional: true},
								"nested_default": {
									Type:     schema.TypeString,
									Optional: true,
									Default:  "old-nested",
								},
							},
						},
					},
				},
			},
		},
	}
	bp1 := pulcheck.BridgedProvider(t, "prov", withNested)

	withoutNested := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"items": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"name":           {Type: schema.TypeString, Optional: true},
								"nested_default": {Type: schema.TypeString, Optional: true},
							},
						},
					},
				},
			},
		},
	}
	bp2 := pulcheck.BridgedProvider(t, "prov", withoutNested)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      items:
        - name: "alpha"
        - name: "beta"
`

	pt := pulcheck.PulCheck(t, bp1, program)
	pt.Up(t)
	stack := pt.ExportStack(t)

	pt2 := pulcheck.PulCheck(t, bp2, program)
	pt2.ImportStack(t, stack)
	preInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	preItems, _ := preInputs["items"].([]any)
	require.Lenf(t, preItems, 2, "sanity: imported state should hold two array elements; got %+v", preInputs)

	res := pt2.Preview(t, optpreview.Diff())
	assertOnlySameOrUpdate(t, res.ChangeSummary, "TypeListOfBlocks")
	pt2.Up(t)

	// Strong assertion: stripArrayOfBlocks recursed into each element and removed the
	// stale nested default. Both elements must survive (positional identity is
	// preserved for TypeList) and neither must carry the stripped field.
	postInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	postItems, ok := postInputs["items"].([]any)
	require.Truef(t, ok, "items must remain an array in stored inputs; got %+v", postInputs)
	require.Lenf(t, postItems, 2, "both array elements must survive; got %+v", postItems)
	for i, elem := range postItems {
		obj, ok := elem.(map[string]any)
		require.Truef(t, ok, "element %d must be an object; got %+v", i, elem)
		_, hasNested := obj["nestedDefault"]
		require.Falsef(t, hasNested,
			"element %d must have nestedDefault stripped; got %+v", i, obj)
	}
}

// TestStripStaleDefaultsIntegration_BridgeDefaultPreserved verifies that fields with
// a bridge-managed default (SchemaInfo.Default) are NOT stripped on Diff — Check is
// responsible for those, and stripping them would interfere with auto-naming and
// similar bridge-side defaults.
func TestStripStaleDefaultsIntegration_BridgeDefaultPreserved(t *testing.T) {
	t.Parallel()

	prov := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
	}
	resourceInfo := map[string]*info.Resource{
		"prov_test": {
			Fields: map[string]*info.Schema{
				"name": {Default: &info.Default{Value: "bridge-managed"}},
			},
		},
	}
	bp := pulcheck.BridgedProvider(t, "prov", prov, pulcheck.WithResourceInfo(resourceInfo))

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
`

	pt := pulcheck.PulCheck(t, bp, program)
	pt.Up(t)

	// A no-op Preview must report only "same" — the bridge default for "name" must
	// survive Diff with __defaults intact (otherwise Check would re-add it on the next
	// run, but a stripped Diff could spuriously report an in-place update).
	res := pt.Preview(t, optpreview.Diff())
	assertNoChanges(t, res.ChangeSummary, "BridgeDefaultPreserved")
}

// TestStripStaleDefaultsIntegration_TypeSetHashStability is the regression guard for
// the bug hit during development of this feature:
// stripping nested fields from TypeSet elements changed element hashes and made TF
// report set rearrangement instead of in-place updates.
// The strip skips TypeSet entirely, so __defaults inside set elements remain intact
// across the round-trip; this test verifies no spurious diff from that round-trip.
func TestStripStaleDefaultsIntegration_TypeSetHashStability(t *testing.T) {
	t.Parallel()

	prov := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"items": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"name": {
									Type:     schema.TypeString,
									Optional: true,
								},
								"nested_default": {
									Type:     schema.TypeString,
									Optional: true,
									Default:  "default-value",
								},
							},
						},
					},
				},
			},
		},
	}
	bp := pulcheck.BridgedProvider(t, "prov", prov)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      items:
        - name: "alpha"
        - name: "beta"
`

	pt := pulcheck.PulCheck(t, bp, program)
	pt.Up(t)

	res := pt.Preview(t, optpreview.Diff())
	assertNoChanges(t, res.ChangeSummary, "TypeSetHashStability")
}
