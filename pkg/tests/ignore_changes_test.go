package tests

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestIgnoreChangesCollections(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                string
		itemsSchema         *schema.Schema
		cloudVal            interface{}
		programVal          string
		program2Val         string
		ignoreChanges       string
		expected            map[string]*pulumirpc.PropertyDiff
		expectedDiffChanges pulumirpc.DiffResponse_DiffChanges
	}{
		{
			name: "ListIndexNestedField",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"weight": {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			programVal:    `[{"weight": 100 }]`,
			cloudVal:      []map[string]interface{}{{"weight": 200}},
			ignoreChanges: `["items[0].weight"]`,
		},
		{
			name: "SetIndexNestedField",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"weight": {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			programVal:    `[{"weight": 100 }]`,
			cloudVal:      []map[string]interface{}{{"weight": 200}},
			ignoreChanges: `["items[0].weight"]`,
		},
		{
			name: "ListIndexNestedFieldWildcard",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"weight": {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			programVal: `[{"weight": 100 }]`,
			cloudVal:   []map[string]interface{}{{"weight": 200}},
			// NOTE: Wildcards work when the Pulumi engine is involved!
			ignoreChanges: `["items[*].weight"]`,
		},
		{
			name: "ListIndexNestedFieldWildcard",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"weight": {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			programVal: `[{"weight": 100 }]`,
			cloudVal:   []map[string]interface{}{{"weight": 200}},
			// NOTE: Wildcards work when the Pulumi engine is involved!
			ignoreChanges: `["items[*].weight"]`,
		},
		{
			name: "ListIndexNestedFieldWildcardAddition",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"weight": {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			programVal:    `[{"weight": 100}]`,
			program2Val:   `[{"weight": 100}, {"weight": 300}]`,
			cloudVal:      []map[string]interface{}{{"weight": 200}},
			ignoreChanges: `["items[*].weight"]`,
			// FIXME: ignoreChanges ignored!
			// TODO[pulumi/pulumi#20447]
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items[0].weight": &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
				"items[1]":        &pulumirpc.PropertyDiff{},
			},
		},
		{
			name: "SetIndexNestedFieldWildcardAddition",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"weight": {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			programVal:    `[{"weight": 100}]`,
			program2Val:   `[{"weight": 100}, {"weight": 300}]`,
			cloudVal:      []map[string]interface{}{{"weight": 200}},
			ignoreChanges: `["items[*].weight"]`,
			// FIXME: ignoreChanges ignored!
			// TODO[pulumi/pulumi#20447]
			expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
			expected: map[string]*pulumirpc.PropertyDiff{
				"items[0].weight": &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
				"items[1]":        &pulumirpc.PropertyDiff{},
			},
		},
		{
			name: "ObjectNestedFieldWildcard",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			programVal:    `{"weight": 100, "other": 100}`,
			cloudVal:      map[string]interface{}{"weight": 200, "other": 200},
			ignoreChanges: `["items.*"]`,
		},
		{
			name: "ObjectNestedFieldWildcardAddition",
			itemsSchema: &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			programVal:    `{"weight": 100, "other": 100}`,
			program2Val:   `{"weight": 100, "other": 100, "third": 300}`,
			cloudVal:      map[string]interface{}{"weight": 200, "other": 200},
			ignoreChanges: `["items.*"]`,
		},
	}
	for _, tc := range testCases {
		opts := []pulcheck.BridgedProviderOpt{}

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resMap := map[string]*schema.Resource{
				"prov_test": {
					Schema: map[string]*schema.Schema{
						"items": tc.itemsSchema,
					},
					ReadContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
						err := rd.Set("items", tc.cloudVal)
						require.NoError(t, err)
						return nil
					},
					CreateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
						rd.SetId("id0")
						return nil
					},
				},
			}

			tfp := &schema.Provider{ResourcesMap: resMap}
			bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, opts...)
			program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    options:
      ignoreChanges: %s
    properties:
      items: %s
`, tc.ignoreChanges, tc.programVal)
			pt := pulcheck.PulCheck(t, bridgedProvider, program)
			pt.Up(t)
			pt.Refresh(t, optrefresh.Diff(), optrefresh.ProgressStreams(os.Stdout), optrefresh.ErrorProgressStreams(os.Stderr))
			if tc.program2Val != "" {
				pt.WritePulumiYaml(t, fmt.Sprintf(`
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    options:
      ignoreChanges: %s
    properties:
      items: %s
`, tc.ignoreChanges, tc.program2Val))
			}
			pt.Preview(t, optpreview.Diff(), optpreview.ProgressStreams(os.Stdout), optpreview.ErrorProgressStreams(os.Stderr))
			diff := extractDiff(t, pt, "mainRes")
			expected := tc.expected
			if tc.expected == nil {
				expected = map[string]*pulumirpc.PropertyDiff{}
			}
			expectedDiffChanges := pulumirpc.DiffResponse_DIFF_NONE
			if tc.expectedDiffChanges != 0 {
				expectedDiffChanges = tc.expectedDiffChanges
			}
			assert.Equal(t, diff.expectedDiffChanges, expectedDiffChanges)
			require.Equal(t, expected, diff.expected)
		})
	}
}

type diffResult struct {
	expected            map[string]*pulumirpc.PropertyDiff
	expectedDiffChanges pulumirpc.DiffResponse_DiffChanges
}

func extractDiff(t *testing.T, pt *pulumitest.PulumiTest, name string) diffResult {
	grpc := pt.GrpcLog(t)
	updates, err := grpc.Diffs()
	require.NoError(t, err)
	for i := range updates {
		u := &updates[i]
		if u.Request.Name == name {
			if u.Response.DetailedDiff == nil {
				u.Response.DetailedDiff = map[string]*pulumirpc.PropertyDiff{}
			} else {
				// we only care about kind for the assertion
				for k, v := range u.Response.DetailedDiff {
					u.Response.DetailedDiff[k] = &pulumirpc.PropertyDiff{
						Kind: v.Kind,
					}
				}
			}
			return diffResult{
				expectedDiffChanges: u.Response.Changes,
				expected:            u.Response.DetailedDiff,
			}
		}
	}
	return diffResult{}
}
