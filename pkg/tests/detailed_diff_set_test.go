package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/require"
)

func runDetailedDiffTest(
	t *testing.T, resMap map[string]*schema.Resource, program1, program2 string,
) (string, map[string]interface{}) {
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())
	pt := pulcheck.PulCheck(t, bridgedProvider, program1)
	pt.Up(t)
	pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

	err := os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
	require.NoError(t, err)

	pt.ClearGrpcLog(t)
	res := pt.Preview(t, optpreview.Diff())
	t.Log(res.StdOut)

	diffResponse := struct {
		DetailedDiff map[string]interface{} `json:"detailedDiff"`
	}{}

	for _, entry := range pt.GrpcLog(t).Entries {
		if entry.Method == "/pulumirpc.ResourceProvider/Diff" {
			err := json.Unmarshal(entry.Response, &diffResponse)
			require.NoError(t, err)
		}
	}

	return res.StdOut, diffResponse.DetailedDiff
}

func TestDetailedDiffSet(t *testing.T) {
	t.Parallel()
	runTest := func(t *testing.T, resMap map[string]*schema.Resource, props1, props2 interface{},
		expected, expectedDetailedDiff autogold.Value,
	) {
		program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s
`
		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)
		program1 := fmt.Sprintf(program, string(props1JSON))
		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)
		program2 := fmt.Sprintf(program, string(props2JSON))
		out, detailedDiff := runDetailedDiffTest(t, resMap, program1, program2)

		expected.Equal(t, trimDiff(t, out))
		expectedDetailedDiff.Equal(t, detailedDiff)
	}

	// The following test cases use the same inputs (props1 and props2) to test a few variations:
	// - The diff when the schema is a set of strings
	// - The diff when the schema is a set of strings with ForceNew
	// - The diff when the schema is a set of structs with a nested string
	// - The diff when the schema is a set of structs with a nested string and ForceNew
	// For each of these variations, we record both the detailed diff output sent to the engine
	// and the output that we expect to see in the Pulumi console.
	type setDetailedDiffTestCase struct {
		name                              string
		props1                            []string
		props2                            []string
		expectedAttrDetailedDiff          autogold.Value
		expectedAttr                      autogold.Value
		expectedAttrForceNewDetailedDiff  autogold.Value
		expectedAttrForceNew              autogold.Value
		expectedBlockDetailedDiff         autogold.Value
		expectedBlock                     autogold.Value
		expectedBlockForceNewDetailedDiff autogold.Value
		expectedBlockForceNew             autogold.Value
	}

	testCases := []setDetailedDiffTestCase{
		{
			name:                              "unchanged",
			props1:                            []string{"val1"},
			props2:                            []string{"val1"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                     "changed non-empty",
			props1:                   []string{"val1"},
			props2:                   []string{"val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nested: "val1" => "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nested: "val1" => "val2"
                }
        ]
`),
		},
		{
			name:                     "changed from empty",
			props1:                   []string{},
			props2:                   []string{"val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: {
              + nested    : "val1"
            }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: {
              + nested    : "val1"
            }
        ]
`),
		},
		{
			name:                     "changed to empty",
			props1:                   []string{"val1"},
			props2:                   []string{},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - nested: "val1"
            }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - nested: "val1"
            }
        ]
`),
		},
		{
			name:                     "removed front",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
		},
		{
			name:                     "removed front unordered",
			props1:                   []string{"val2", "val1", "val3"},
			props2:                   []string{"val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
		},
		{
			name:                     "removed middle",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
		},
		{
			name:                     "removed middle unordered",
			props1:                   []string{"val2", "val3", "val1"},
			props2:                   []string{"val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                     "removed end",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                     "removed end unordered",
			props1:                   []string{"val2", "val3", "val1"},
			props2:                   []string{"val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
		},
		{
			name:                     "added front",
			props1:                   []string{"val2", "val3"},
			props2:                   []string{"val1", "val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
		},
		{
			name:                     "added front unordered",
			props1:                   []string{"val3", "val1"},
			props2:                   []string{"val2", "val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val3" => "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val3" => "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val3" => "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val3" => "val2"
                }
        ]
`),
		},
		{
			name:                     "added middle",
			props1:                   []string{"val1", "val3"},
			props2:                   []string{"val1", "val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
		},
		{
			name:                     "added middle unordered",
			props1:                   []string{"val2", "val1"},
			props2:                   []string{"val2", "val3", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "added end",
			props1:                   []string{"val1", "val2"},
			props2:                   []string{"val1", "val2", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "added end unordered",
			props1:                   []string{"val2", "val3"},
			props2:                   []string{"val2", "val3", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val1"
                }
        ]
`),
		},
		{
			name:                     "same element updated",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val1", "val4", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val2" => "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: "val2" => "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val2" => "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [1]: {
                  ~ nested: "val2" => "val4"
                }
        ]
`),
		},
		{
			name:   "same element updated unordered",
			props1: []string{"val2", "val3", "val1"},
			props2: []string{"val2", "val4", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val4"
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val4"
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val4"
                }
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val4"
                }
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                              "shuffled",
			props1:                            []string{"val1", "val2", "val3"},
			props2:                            []string{"val3", "val1", "val2"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                              "shuffled unordered",
			props1:                            []string{"val2", "val3", "val1"},
			props2:                            []string{"val3", "val1", "val2"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                              "shuffled with duplicates",
			props1:                            []string{"val1", "val2", "val3"},
			props2:                            []string{"val3", "val1", "val2", "val3"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                              "shuffled with duplicates unordered",
			props1:                            []string{"val2", "val3", "val1"},
			props2:                            []string{"val3", "val1", "val2", "val3"},
			expectedAttrDetailedDiff:          autogold.Expect(map[string]interface{}{}),
			expectedAttr:                      autogold.Expect("\n"),
			expectedAttrForceNewDetailedDiff:  autogold.Expect(map[string]interface{}{}),
			expectedAttrForceNew:              autogold.Expect("\n"),
			expectedBlockDetailedDiff:         autogold.Expect(map[string]interface{}{}),
			expectedBlock:                     autogold.Expect("\n"),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{}),
			expectedBlockForceNew:             autogold.Expect("\n"),
		},
		{
			name:                     "shuffled added front",
			props1:                   []string{"val2", "val3"},
			props2:                   []string{"val1", "val3", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val1"
                }
        ]
`),
		},
		{
			name:                     "shuffled added front unordered",
			props1:                   []string{"val3", "val1"},
			props2:                   []string{"val2", "val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val2"
                }
        ]
`),
		},
		{
			name:                     "shuffled added middle",
			props1:                   []string{"val1", "val3"},
			props2:                   []string{"val3", "val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val2"
                }
        ]
`),
		},
		{
			name:                     "shuffled added middle unordered",
			props1:                   []string{"val2", "val1"},
			props2:                   []string{"val1", "val3", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "shuffled added end",
			props1:                   []string{"val1", "val2"},
			props2:                   []string{"val2", "val1", "val3"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
        ]
`),
		},
		{
			name:                     "shuffled removed front",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val3", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: "val1"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[0]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested: "val1"
                }
        ]
`),
		},
		{
			name:                     "shuffled removed middle",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val3", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: "val2"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[1]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [1]: {
                  - nested: "val2"
                }
        ]
`),
		},
		{
			name:                     "shuffled removed end",
			props1:                   []string{"val1", "val2", "val3"},
			props2:                   []string{"val2", "val1"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
        ]
`),
		},
		{
			name:                     "two added",
			props1:                   []string{"val1", "val2"},
			props2:                   []string{"val1", "val2", "val3", "val4"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}, "tests[3]": map[string]interface{}{}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
          + [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: "val3"
          + [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{}, "tests[3]": map[string]interface{}{}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
          + [3]: {
                  + nested    : "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "ADD_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "ADD_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [2]: {
                  + nested    : "val3"
                }
          + [3]: {
                  + nested    : "val4"
                }
        ]
`),
		},
		{
			name:                     "two removed",
			props1:                   []string{"val1", "val2", "val3", "val4"},
			props2:                   []string{"val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}, "tests[3]": map[string]interface{}{"kind": "DELETE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE"}, "tests[3]": map[string]interface{}{"kind": "DELETE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
		},
		{
			name:                     "two added and two removed",
			props1:                   []string{"val1", "val2", "val3", "val4"},
			props2:                   []string{"val1", "val2", "val5", "val6"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE"}}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val5"
          ~ [3]: "val4" => "val6"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3]": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: "val3" => "val5"
          ~ [3]: "val4" => "val6"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE"}}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val5"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val6"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}, "tests[3].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"}}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [2]: {
                  ~ nested: "val3" => "val5"
                }
          ~ [3]: {
                  ~ nested: "val4" => "val6"
                }
        ]
`),
		},
		{
			name:   "two added and two removed shuffled, one overlaps",
			props1: []string{"val1", "val2", "val3", "val4"},
			props2: []string{"val1", "val5", "val6", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
		},
		{
			name:   "two added and two removed shuffled, no overlaps",
			props1: []string{"val1", "val2", "val3", "val4"},
			props2: []string{"val5", "val6", "val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{},
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val5"
          + [1]: "val6"
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: "val5"
          + [1]: "val6"
          - [2]: "val3"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{},
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "DELETE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val5"
                }
          + [1]: {
                  + nested    : "val6"
                }
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[0]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "DELETE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [0]: {
                  + nested    : "val5"
                }
          + [1]: {
                  + nested    : "val6"
                }
          - [2]: {
                  - nested: "val3"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
		},
		{
			name:   "two added and two removed shuffled, with duplicates",
			props1: []string{"val1", "val2", "val3", "val4"},
			props2: []string{"val1", "val5", "val6", "val2", "val1", "val2"},
			expectedAttrDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{},
				"tests[2]": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE"},
			}),
			expectedAttr: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedAttrForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]": map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2]": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]": map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedAttrForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "val5"
          ~ [2]: "val3" => "val6"
          - [3]: "val4"
        ]
`),
			expectedBlockDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE"},
			}),
			expectedBlock: autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
			expectedBlockForceNewDetailedDiff: autogold.Expect(map[string]interface{}{
				"tests[1]":        map[string]interface{}{"kind": "ADD_REPLACE"},
				"tests[2].nested": map[string]interface{}{"kind": "UPDATE_REPLACE"},
				"tests[3]":        map[string]interface{}{"kind": "DELETE_REPLACE"},
			}),
			expectedBlockForceNew: autogold.Expect(`
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + nested    : "val5"
                }
          ~ [2]: {
                  ~ nested: "val3" => "val6"
                }
          - [3]: {
                  - nested: "val4"
                }
        ]
`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, forceNew := range []bool{false, true} {
				t.Run(fmt.Sprintf("ForceNew=%v", forceNew), func(t *testing.T) {
					t.Parallel()
					expected := tc.expectedAttr
					if forceNew {
						expected = tc.expectedAttrForceNew
					}

					expectedDetailedDiff := tc.expectedAttrDetailedDiff
					if forceNew {
						expectedDetailedDiff = tc.expectedAttrForceNewDetailedDiff
					}
					t.Run("Attribute", func(t *testing.T) {
						res := &schema.Resource{
							Schema: map[string]*schema.Schema{
								"test": {
									Type:     schema.TypeSet,
									Optional: true,
									Elem: &schema.Schema{
										Type: schema.TypeString,
									},
									ForceNew: forceNew,
								},
							},
						}
						runTest(t, map[string]*schema.Resource{"prov_test": res}, tc.props1, tc.props2, expected, expectedDetailedDiff)
					})

					expected = tc.expectedBlock
					if forceNew {
						expected = tc.expectedBlockForceNew
					}
					expectedDetailedDiff = tc.expectedBlockDetailedDiff
					if forceNew {
						expectedDetailedDiff = tc.expectedBlockForceNewDetailedDiff
					}

					t.Run("Block", func(t *testing.T) {
						res := &schema.Resource{
							Schema: map[string]*schema.Schema{
								"test": {
									Type:     schema.TypeSet,
									Optional: true,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"nested": {
												Type:     schema.TypeString,
												Optional: true,
												ForceNew: forceNew,
											},
										},
									},
								},
							},
						}

						props1 := make([]interface{}, len(tc.props1))
						for i, v := range tc.props1 {
							props1[i] = map[string]interface{}{"nested": v}
						}

						props2 := make([]interface{}, len(tc.props2))
						for i, v := range tc.props2 {
							props2[i] = map[string]interface{}{"nested": v}
						}

						runTest(t, map[string]*schema.Resource{"prov_test": res}, props1, props2, expected, expectedDetailedDiff)
					})
				})
			}
		})
	}
}

// "UNKNOWN" for unknown values
func testDetailedDiffWithUnknowns(t *testing.T, resMap map[string]*schema.Resource, unknownString string, props1, props2 interface{}, expected, expectedDetailedDiff autogold.Value) {
	originalProgram := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
		tests: %s
outputs:
  testOut: ${mainRes.tests}
	`
	props1JSON, err := json.Marshal(props1)
	require.NoError(t, err)
	program1 := fmt.Sprintf(originalProgram, string(props1JSON))

	programWithUnknown := `
name: test
runtime: yaml
resources:
  auxRes:
    type: prov:index:Aux
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s
outputs:
  testOut: ${mainRes.tests}
`
	props2JSON, err := json.Marshal(props2)
	require.NoError(t, err)
	program2 := fmt.Sprintf(programWithUnknown, string(props2JSON))
	program2 = strings.ReplaceAll(program2, "UNKNOWN", unknownString)

	out, detailedDiff := runDetailedDiffTest(t, resMap, program1, program2)
	expected.Equal(t, trimDiff(t, out))
	expectedDetailedDiff.Equal(t, detailedDiff)
}

func TestDetailedDiffUnknownSetAttributeElement(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeString,
					Computed: true,
					Optional: true,
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", "aux")
				require.NoError(t, err)
				return nil
			},
		},
	}

	t.Run("empty to unknown element", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{},
			[]interface{}{"UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: output<string>
        ]
    --outputs:--
  + testOut: output<string>
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}))
	})

	t.Run("non-empty to unknown element", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1"},
			[]interface{}{"UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => output<string>
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}))
	})

	t.Run("unknown element added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val2", "val3"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val2" => output<string>
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
          ~ [1]: "val3" => output<string>
          + [2]: "val3"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val1", "val2", "UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
            [1]: "val2"
          + [2]: output<string>
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("element updated to unknown", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
          ~ [1]: "val2" => output<string>
            [2]: "val3"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val3", "val2"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val2" => output<string>
            [1]: "val3"
          + [2]: "val2"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val3", "UNKNOWN", "val1"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val3"
          ~ [1]: "val3" => output<string>
          + [2]: "val1"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val2", "val1", "UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
          ~ [1]: "val2" => "val1"
          + [2]: output<string>
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

func TestUnknownSetAttributeDiff(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeSet,
					Computed: true,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", []interface{}{"aux"})
				require.NoError(t, err)
				return nil
			},
		},
	}

	t.Run("empty to unknown set", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.auxes}",
			[]interface{}{},
			"UNKNOWN",
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: output<string>
    --outputs:--
  + testOut: output<string>
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
		)
	})

	t.Run("non-empty to unknown set", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.auxes}",
			[]interface{}{"val"},
			"UNKNOWN",
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val"
        ]
      + tests: output<string>
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

func TestDetailedDiffSetDuplicates(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, `["a", "a"]`))
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, `["b", "b", "a", "a", "c"]`))

		res := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "b"
          + [4]: "c"
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test = ["a", "a"]
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test = ["b", "b", "a", "a", "c"]
}`)

		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id   = "newid"
      ~ test = [
          + "b",
          + "c",
            # (1 unchanged element hidden)
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestDetailedDiffSetNestedAttributeUpdated(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"nested2": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"nested3": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
		},
	}

	tfp := &schema.Provider{ResourcesMap: resMap}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		props1 := []map[string]string{
			{"nested": "b", "nested2": "b", "nested3": "b"},
			{"nested": "a", "nested2": "a", "nested3": "a"},
			{"nested": "c", "nested2": "c", "nested3": "c"},
		}
		props2 := []map[string]string{
			{"nested": "b", "nested2": "b", "nested3": "b"},
			{"nested": "d", "nested2": "a", "nested3": "a"},
			{"nested": "c", "nested2": "c", "nested3": "c"},
		}

		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, string(props1JSON)))
		pt.Up(t)

		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, string(props2JSON)))

		res := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested : "a"
                  - nested2: "a"
                  - nested3: "a"
                }
          + [1]: {
                  + nested    : "d"
                  + nested2   : "a"
                  + nested3   : "a"
                }
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "b"
	  nested2 = "b"
	  nested3 = "b"
	}
 test {
	  nested = "a"	
	  nested2 = "a"
	  nested3 = "a"
	}
 test {
	  nested = "c"
	  nested2 = "c"
	  nested3 = "c"
	}
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "b"
	  nested2 = "b"
	  nested3 = "b"
	}
 test {
	  nested = "d"	
	  nested2 = "a"
	  nested3 = "a"
	}
 test {
	  nested = "c"
	  nested2 = "c"
	  nested3 = "c"
	}
}`)

		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id = "newid"

      - test {
          - nested  = "a" -> null
          - nested2 = "a" -> null
          - nested3 = "a" -> null
        }
      + test {
          + nested  = "d"
          + nested2 = "a"
          + nested3 = "a"
        }

        # (2 unchanged blocks hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestDetailedDiffSetComputedNestedAttribute(t *testing.T) {
	t.Parallel()
	resCount := 0
	setComputedProp := func(t *testing.T, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
		testSet := d.Get("test").(*schema.Set)
		testVals := testSet.List()
		newTestVals := make([]interface{}, len(testVals))
		for i, v := range testVals {
			val := v.(map[string]interface{})
			if val["computed"] == nil || val["computed"] == "" {
				val["computed"] = fmt.Sprint(resCount)
				resCount++
			}
			newTestVals[i] = val
		}

		err := d.Set("test", schema.NewSet(testSet.F, newTestVals))
		require.NoError(t, err)
		return nil
	}

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"computed": {
								Type:     schema.TypeString,
								Optional: true,
								Computed: true,
							},
						},
					},
				},
			},
			CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				d.SetId("id")
				return setComputedProp(t, d, i)
			},
			UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				return setComputedProp(t, d, i)
			},
		},
	}

	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		props1 := []map[string]string{
			{"nested": "a", "computed": "b"},
		}
		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, string(props1JSON)))
		pt.Up(t)

		props2 := []map[string]string{
			{"nested": "a"},
			{"nested": "a", "computed": "b"},
		}
		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, string(props2JSON)))
		res := pt.Preview(t, optpreview.Diff())

		// TODO[pulumi/pulumi-terraform-bridge#2528]: The preview is wrong here because of the computed property
		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=id]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + computed  : "b"
                  + nested    : "a"
                }
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		resCount = 0
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "a"
	  computed = "b"
	}
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "a"
	  computed = "b"
	}
  test {
	  nested = "a"
	}
}`)
		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id = "id"

      + test {
          + computed = (known after apply)
          + nested   = "a"
        }

        # (1 unchanged block hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}
