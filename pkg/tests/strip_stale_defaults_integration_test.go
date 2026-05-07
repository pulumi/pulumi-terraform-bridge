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
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// Tests in this file exercise stripStaleDefaults through the runtime SDKv2 paths
// (Diff RPC and Update's internal tf.Diff), catching interactions with TF's
// plan/hash/detailed-diff machinery that the unit tests in pkg/tfbridge can't see.
//
// The stub TF providers used here don't run validation, so a "no error"
// assertion would pass even with the strip deleted. Tests use one of two
// stronger assertions instead:
//   - assertNoChanges for no-op scenarios (unchanged schema, unchanged program):
//     the preview must report only "same".
//   - resourceInputs checks for schema-migration scenarios (provider swapped
//     between Up calls): inspect the post-Up exported stack to confirm the
//     stale field was removed.

// assertNoChanges fails if the preview reports any op other than "same". Use on
// no-op scenarios to catch spurious diffs caused by inputs that don't round-trip
// through TF cleanly.
func assertNoChanges(t *testing.T, summary map[apitype.OpType]int, label string) {
	t.Helper()
	for op, count := range summary {
		require.Equalf(t, apitype.OpSame, op,
			"%s: preview must report only 'same' resources; got %d %q ops (full summary: %+v)",
			label, count, op, summary)
	}
}

// assertNoUnexpectedOps fails if the preview reports any op other than "same"
// or "update". Use on schema-migration scenarios where an in-place update is
// expected; a replace would indicate the strip caused unintended set-hash or
// identity changes. Does not require the summary to be non-empty.
func assertNoUnexpectedOps(t *testing.T, summary map[apitype.OpType]int, label string) {
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
	assertNoUnexpectedOps(t, res.ChangeSummary, "DefaultRemoved")
	pt2.Up(t)

	// Without the strip, Check's "old default" reuse path would re-pin
	// "old-default" into news on every Diff and post-Up inputs would still
	// record optField — so absence here proves the strip ran.
	postInputs := resourceInputs(t, pt2.ExportStack(t).Deployment, "prov:index/test:Test::mainRes")
	_, hasField := postInputs["optField"]
	require.False(t, hasField,
		"after Up against the schema with no Default, optField must be absent from stored inputs; got %+v", postInputs)
}

// Note: there is no integration test for the *changed-default* scenario (provider
// upgrade where a TF schema Default goes from v1 to v2). The changed-default case
// is a known phantom-diff limitation tracked by issue #3434; resolving it without
// regressing the legacy-stack falsy-default round-trip invariant
// (TestUpdatePreservesLegacyFalsyTFDefaults) requires the architectural cleanup
// described there.

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
	assertNoUnexpectedOps(t, res.ChangeSummary, "FieldRemovedFromSchema")
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
	assertNoUnexpectedOps(t, res.ChangeSummary, "NestedTypeListBlock")
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
	assertNoUnexpectedOps(t, res.ChangeSummary, "TypeListOfBlocks")
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

// TestStripStaleDefaultsIntegration_StripAppliesInUpdatePath is a regression
// guard for strip symmetry across the two RPC paths that feed PlanResourceChange
// (Diff RPC and Update's internal tf.Diff). Both must strip before building the
// TF config; if Update drifts, Preview and Apply diverge.
//
// Check sanitizes news upstream today, so the test passes even if Update's
// strip is removed. The value is forward-looking: a future path that leaks
// stale entries past Check (custom state-edit hooks, new RPC paths, refresh
// flows that bypass Check) would be caught only if both Diff and Update strip.
// CustomizeDiff records every PlanResourceChange site that sees opt_field; the
// assertion is zero.
func TestStripStaleDefaultsIntegration_StripAppliesInUpdatePath(t *testing.T) {
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
					"trigger_update": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
	}
	bp1 := pulcheck.BridgedProvider(t, "prov", withDefault)

	// staleSeen records every CustomizeDiff invocation where opt_field appears in the
	// raw config. The expected count under the fix is zero — both Diff RPC and Update
	// RPC must strip before PlanResourceChange runs.
	var staleSeen atomic.Int64
	var seenLock sync.Mutex
	var seenSites []string
	recordSite := func(value any) {
		seenLock.Lock()
		defer seenLock.Unlock()
		seenSites = append(seenSites, formatVal(value))
	}

	withoutDefault := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"opt_field": {
						Type:     schema.TypeString,
						Optional: true,
						// Default removed.
					},
					"trigger_update": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
				CustomizeDiff: func(_ context.Context, d *schema.ResourceDiff, _ any) error {
					raw := d.GetRawConfig()
					if raw.IsNull() || !raw.Type().IsObjectType() || !raw.Type().HasAttribute("opt_field") {
						return nil
					}
					optAttr := raw.GetAttr("opt_field")
					if !optAttr.IsNull() && optAttr.IsKnown() {
						staleSeen.Add(1)
						recordSite(optAttr.AsString())
					}
					return nil
				},
			},
		},
	}
	bp2 := pulcheck.BridgedProvider(t, "prov", withoutDefault)

	// Trigger an Update by changing trigger_update across the upgrade — this forces the
	// engine to call Update (not just Diff) so that Update's internal tf.Diff exercises
	// the strip path under test.
	programV1 := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      triggerUpdate: "before"
`
	programV2 := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      triggerUpdate: "after"
`

	pt := pulcheck.PulCheck(t, bp1, programV1)
	pt.Up(t)
	stack := pt.ExportStack(t)

	pt2 := pulcheck.PulCheck(t, bp2, programV2)
	pt2.ImportStack(t, stack)
	pt2.Up(t)

	require.Zerof(t, staleSeen.Load(),
		"opt_field must not reach PlanResourceChange after the strip; saw it at: %v", seenSites)
}

// formatVal renders a cty value into a short string for diagnostic output. Defined
// locally because the test only needs a best-effort representation.
func formatVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "<non-string>"
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
