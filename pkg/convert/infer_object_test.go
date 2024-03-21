package convert

import (
	"fmt"
	"testing"

	"github.com/hexops/autogold/v2"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestInferType(t *testing.T) {
	type testCase struct {
		name   string
		schema *schema.Schema
		expect autogold.Value
	}
	testCases := []testCase{
		{"bool", &schema.Schema{Type: shim.TypeBool}, autogold.Expect("tftypes.Bool")},
		{"string", &schema.Schema{Type: shim.TypeString}, autogold.Expect("tftypes.String")},
		{"int", &schema.Schema{Type: shim.TypeInt}, autogold.Expect("tftypes.Number")},
		{"float", &schema.Schema{Type: shim.TypeFloat}, autogold.Expect("tftypes.Number")},
	}

	type element struct {
		name string
		elem interface{}
	}

	var res shim.Resource = (&schema.Resource{
		Schema: &schema.SchemaMap{
			"x": (&schema.Schema{Type: shim.TypeString}).Shim(),
			"y": (&schema.Schema{Type: shim.TypeInt, Optional: true}).Shim(),
		},
	}).Shim()

	elements := []element{
		{"nil", nil},
		{"int", (&schema.Schema{Type: shim.TypeInt}).Shim()},
		{"res", res},
	}

	expected := map[string]autogold.Value{
		"Set-nil":  autogold.Expect("tftypes.Set[tftypes.Object[]]"),
		"Set-int":  autogold.Expect("tftypes.Set[tftypes.Number]"),
		"Set-res":  autogold.Expect(`tftypes.Set[tftypes.Object["x":tftypes.String, "y":tftypes.Number]]`),
		"List-nil": autogold.Expect("tftypes.List[tftypes.Object[]]"),
		"List-int": autogold.Expect("tftypes.List[tftypes.Number]"),
		"List-res": autogold.Expect(`tftypes.List[tftypes.Object["x":tftypes.String, "y":tftypes.Number]]`),
		"Map-nil":  autogold.Expect("tftypes.Map[tftypes.Object[]]"),
		"Map-int":  autogold.Expect("tftypes.Map[tftypes.Number]"),
		// Not intuitive but represents a single-nested block, see docs on shim.Schema.Elem().
		"Map-res": autogold.Expect(`tftypes.Object["x":tftypes.String, "y":tftypes.Number]`),
	}

	for _, con := range []shim.ValueType{shim.TypeSet, shim.TypeList, shim.TypeMap} {
		for _, el := range elements {
			name := fmt.Sprintf("%v-%s", con, el.name)
			if _, ok := expected[name]; !ok {
				panic(fmt.Sprintf("Missing expectation for %q", name))
			}
			tc := testCase{
				name: name,
				schema: &schema.Schema{
					Type: con,
					Elem: el.elem,
				},
				expect: expected[name],
			}
			testCases = append(testCases, tc)
		}
	}

	testCases = append(testCases, testCase{
		"invalid",
		&schema.Schema{Type: shim.TypeInvalid}, autogold.Expect("tftypes.Object[]"),
	})

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := InferType(tc.schema.Shim(), nil)
			tc.expect.Equal(t, got.String())
		})
	}
}
