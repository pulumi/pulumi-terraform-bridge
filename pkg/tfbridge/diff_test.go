package tfbridge

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	v2Schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

type DiffKind = pulumirpc.PropertyDiff_Kind

const (
	A  = pulumirpc.PropertyDiff_ADD
	D  = pulumirpc.PropertyDiff_DELETE
	U  = pulumirpc.PropertyDiff_UPDATE
	AR = pulumirpc.PropertyDiff_ADD_REPLACE
	DR = pulumirpc.PropertyDiff_DELETE_REPLACE
	UR = pulumirpc.PropertyDiff_UPDATE_REPLACE
)

var computedValue = resource.Computed{Element: resource.NewStringProperty("")}

func TestCustomizeDiff(t *testing.T) {
	inputsMap := resource.NewPropertyMapFromMap(map[string]interface{}{
		"prop": "foo",
	})
	stateMap := resource.NewPropertyMapFromMap(map[string]interface{}{
		"prop": "foo",
		"outp": false,
	})

	// Pulumi Schema
	info := map[string]*SchemaInfo{}
	tfs := map[string]*v2Schema.Schema{
		"prop": {Type: v2Schema.TypeString},
		"outp": {Type: v2Schema.TypeBool, Computed: true},
	}
	// Terraform schema
	sch := shimv2.NewSchemaMap(tfs)

	// ignores
	var ignores []string

	t.Run("CustomDiffCausesAddReplace", func(t *testing.T) {
		ctx := context.Background()
		// expected diff
		expected := map[string]DiffKind{
			"outp": AR,
		}

		// Fake up a TF resource and a TF provider.
		customDiffRes := &v2Schema.Resource{
			Schema: tfs,
			CustomizeDiff: func(_ context.Context, d *v2Schema.ResourceDiff, _ interface{}) error {
				var err error
				err = d.SetNew("outp", true)
				if err != nil {
					return err
				}
				err = d.ForceNew("outp")
				if err != nil {
					return err
				}
				return err
			},
		}
		provider := shimv2.NewProvider(&v2Schema.Provider{
			ResourcesMap: map[string]*v2Schema.Resource{
				"resource": customDiffRes,
			},
		})

		// Convert the inputs and state to TF config and resource attributes.
		r := Resource{
			TF:     shimv2.NewResource(customDiffRes),
			Schema: &ResourceInfo{Fields: info},
		}
		tfState, err := makeTerraformStateWithOpts(ctx, r, "id", stateMap, makeTerraformStateOpts{})
		assert.NoError(t, err)

		config, _, err := MakeTerraformConfig(ctx, &Provider{tf: provider}, inputsMap, sch, info)
		assert.NoError(t, err)

		tfDiff, err := provider.Diff(ctx, "resource", tfState, config, shim.DiffOptions{
			IgnoreChanges: newIgnoreChanges(ctx, sch, info, stateMap, inputsMap, ignores),
		})
		assert.NoError(t, err)

		// Convert the diff to a detailed diff and check the result.
		diff, changes := makeDetailedDiff(ctx, sch, info, stateMap, inputsMap, tfDiff)
		expectedDiff := map[string]*pulumirpc.PropertyDiff{}
		for k, v := range expected {
			expectedDiff[k] = &pulumirpc.PropertyDiff{Kind: v}
		}
		assert.Equal(t, pulumirpc.DiffResponse_DIFF_SOME, changes)
		assert.Equal(t, expectedDiff, diff)
	})

	t.Run("NoCustomDiffCausesNoDiff", func(t *testing.T) {
		ctx := context.Background()
		// expected diff
		expected := map[string]DiffKind{}

		// Fake up a TF resource and a TF provider.
		noCustomDiffRes := &v2Schema.Resource{
			Schema: tfs,
		}
		provider := shimv2.NewProvider(&v2Schema.Provider{
			ResourcesMap: map[string]*v2Schema.Resource{
				"resource": noCustomDiffRes,
			},
		})

		// Convert the inputs and state to TF config and resource attributes.
		r := Resource{
			TF:     shimv2.NewResource(noCustomDiffRes),
			Schema: &ResourceInfo{Fields: info},
		}
		tfState, err := makeTerraformStateWithOpts(ctx, r, "id", stateMap, makeTerraformStateOpts{})
		assert.NoError(t, err)

		config, _, err := MakeTerraformConfig(ctx, &Provider{tf: provider}, inputsMap, sch, info)
		assert.NoError(t, err)

		tfDiff, err := provider.Diff(ctx, "resource", tfState, config, shim.DiffOptions{
			IgnoreChanges: newIgnoreChanges(ctx, sch, info, stateMap, inputsMap, ignores),
		})
		assert.NoError(t, err)

		// Convert the diff to a detailed diff and check the result.
		diff, changes := makeDetailedDiff(ctx, sch, info, stateMap, inputsMap, tfDiff)
		expectedDiff := map[string]*pulumirpc.PropertyDiff{}
		for k, v := range expected {
			expectedDiff[k] = &pulumirpc.PropertyDiff{Kind: v}
		}
		assert.Equal(t, changes, pulumirpc.DiffResponse_DIFF_NONE)
		assert.Equal(t, expectedDiff, diff)
	})

	t.Run("CustomDiffDoesNotPanicOnGetRawStateOrRawConfig", func(t *testing.T) {
		for _, diffStrat := range []shimv2.DiffStrategy{shimv2.PlanState, shimv2.ClassicDiff} {
			diffStrat := diffStrat
			t.Run(fmt.Sprintf("%v", diffStrat), func(t *testing.T) {
				ctx := context.Background()
				customDiffRes := &v2Schema.Resource{
					Schema: tfs,
					CustomizeDiff: func(_ context.Context, diff *v2Schema.ResourceDiff, _ interface{}) error {
						rawStateType := diff.GetRawState().Type()
						if !rawStateType.HasAttribute("outp") {
							return fmt.Errorf("Expected rawState type to have attribute: outp")
						}
						rawConfigType := diff.GetRawConfig().Type()
						if !rawConfigType.HasAttribute("outp") {
							return fmt.Errorf("Expected rawConfig type to have attribute: outp")
						}
						return nil
					},
				}

				v2Provider := &v2Schema.Provider{
					ResourcesMap: map[string]*v2Schema.Resource{
						"resource": customDiffRes,
					},
				}

				provider := shimv2.NewProvider(v2Provider, shimv2.WithDiffStrategy(diffStrat))

				// Convert the inputs and state to TF config and resource attributes.
				r := Resource{
					TF:     shimv2.NewResource(customDiffRes),
					Schema: &ResourceInfo{Fields: info},
				}
				tfState, err := makeTerraformStateWithOpts(ctx, r, "id", stateMap, makeTerraformStateOpts{})
				assert.NoError(t, err)

				config, _, err := MakeTerraformConfig(ctx, &Provider{tf: provider}, inputsMap, sch, info)
				assert.NoError(t, err)

				// Calling Diff with the given CustomizeDiff used to panic, no more asserts needed.
				_, err = provider.Diff(ctx, "resource", tfState, config, shim.DiffOptions{})
				assert.NoError(t, err)
			})
		}
	})
}

func diffTest(t *testing.T, tfs map[string]*schema.Schema, info map[string]*SchemaInfo,
	inputs, state map[string]interface{}, expected map[string]DiffKind,
	expectedDiffChanges pulumirpc.DiffResponse_DiffChanges,
	ignoreChanges ...string) {
	ctx := context.Background()

	inputsMap := resource.NewPropertyMapFromMap(inputs)
	stateMap := resource.NewPropertyMapFromMap(state)

	sch := shimv1.NewSchemaMap(tfs)

	// Fake up a TF resource and a TF provider.
	res := &schema.Resource{
		Schema: tfs,
		CustomizeDiff: func(d *schema.ResourceDiff, _ interface{}) error {
			return d.SetNewComputed("outp")
		},
	}
	provider := shimv1.NewProvider(&schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"resource": res,
		},
	})

	// Convert the inputs and state to TF config and resource attributes.
	r := Resource{
		TF:     shimv1.NewResource(res),
		Schema: &ResourceInfo{Fields: info},
	}
	tfState, err := makeTerraformStateWithOpts(ctx, r, "id", stateMap, makeTerraformStateOpts{})
	assert.NoError(t, err)

	config, _, err := MakeTerraformConfig(ctx, &Provider{tf: provider}, inputsMap, sch, info)
	assert.NoError(t, err)

	t.Run("standard", func(t *testing.T) {
		tfDiff, err := provider.Diff(ctx, "resource", tfState, config, shim.DiffOptions{
			IgnoreChanges: newIgnoreChanges(ctx, sch, info, stateMap, inputsMap, ignoreChanges),
		})
		assert.NoError(t, err)

		// Convert the diff to a detailed diff and check the result.
		diff, changes := makeDetailedDiff(ctx, sch, info, stateMap, inputsMap, tfDiff)
		expectedDiff := map[string]*pulumirpc.PropertyDiff{}
		for k, v := range expected {
			expectedDiff[k] = &pulumirpc.PropertyDiff{Kind: v}
		}
		assert.Equal(t, expectedDiffChanges, changes)
		assert.Equal(t, expectedDiff, diff)
	})

	// Add an ignoreChanges entry for each path in the expected diff, then re-convert the diff
	// and check the result.
	t.Run("withIgnoreAllExpected", func(t *testing.T) {
		for k := range expected {
			ignoreChanges = append(ignoreChanges, k)
		}
		tfDiff, err := provider.Diff(ctx, "resource", tfState, config, shim.DiffOptions{
			IgnoreChanges: newIgnoreChanges(ctx, sch, info, stateMap, inputsMap, ignoreChanges),
		})
		assert.NoError(t, err)

		diff, changes := makeDetailedDiff(ctx, sch, info, stateMap, inputsMap, tfDiff)
		assert.Equal(t, pulumirpc.DiffResponse_DIFF_NONE, changes)
		assert.Equal(t, map[string]*pulumirpc.PropertyDiff{}, diff)
	})
}

func TestCustomDiffProducesReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE)
}

func TestEmptyDiff(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE)
}

func TestSimpleAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString, Optional: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSimpleAddReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString, Optional: true, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSimpleDelete(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString, Optional: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": D,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSimpleDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString, Optional: true, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSimpleUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSimpleUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop.nest": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedAddReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop.nest": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedDelete(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop.nest": D,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop.nest": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "baz"},
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop.nest": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "baz"},
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop.nest": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedIgnore(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "baz"},
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE,
		"prop")
}

func TestListAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListAddReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListDelete(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": D,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListIgnore(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE,
		"prop")
}

func TestMaxItemsOneListAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"prop": nil,
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneListAddReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"prop": nil,
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneListDelete(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": D,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneListDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneListUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneListUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneListIgnore(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE,
		"prop")
}

func TestSetAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Set:  func(_ interface{}) int { return 0 },
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetAddReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Set:      func(_ interface{}) int { return 0 },
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetDelete(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Set:  func(_ interface{}) int { return 0 },
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": D,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Set:      func(_ interface{}) int { return 0 },
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetIgnore(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE,
		"prop")
}

func TestSetUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0]": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneSetAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Set:      func(_ interface{}) int { return 0 },
				Elem:     &schema.Schema{Type: schema.TypeString},
				MaxItems: 1,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"prop": nil,
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneSetAddReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Set:      func(_ interface{}) int { return 0 },
				Elem:     &schema.Schema{Type: schema.TypeString},
				MaxItems: 1,
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "foo",
		},
		map[string]interface{}{
			"prop": nil,
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneSetDelete(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Set:      func(_ interface{}) int { return 0 },
				Elem:     &schema.Schema{Type: schema.TypeString},
				MaxItems: 1,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": D,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneSetDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Set:      func(_ interface{}) int { return 0 },
				Elem:     &schema.Schema{Type: schema.TypeString},
				MaxItems: 1,
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneSetUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestMaxItemsOneSetIgnore(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE,
		"prop")
}

func TestMaxItemsOneSetUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true, MaxItems: 1},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": "baz",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetNestedUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "baz"}},
		},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0].nest": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetNestedUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true, ForceNew: true},
					},
				},
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "baz"}},
		},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0].nest": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetNestedIgnore(t *testing.T) {
	for _, ignore := range []string{"prop[0]", "prop"} {
		diffTest(t,
			map[string]*schema.Schema{
				"prop": {
					Type: schema.TypeSet,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nest": {Type: schema.TypeString, Required: true},
						},
					},
				},
				"outp": {Type: schema.TypeString, Computed: true},
			},
			map[string]*SchemaInfo{},
			map[string]interface{}{
				"prop": []interface{}{map[string]interface{}{"nest": "baz"}},
			},
			map[string]interface{}{
				"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
				"outp": "bar",
			},
			map[string]DiffKind{},
			pulumirpc.DiffResponse_DIFF_NONE,
			ignore)
	}
}

func TestComputedSimpleUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": computedValue,
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedSimpleUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeString, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": computedValue,
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedMapUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": computedValue,
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedNestedUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": computedValue},
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedNestedUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": computedValue},
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedNestedIgnore(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeMap},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": computedValue},
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE,
		"prop")
}

func TestComputedListUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": computedValue,
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedListElementUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedListElementUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedListElementIgnore(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{},
		pulumirpc.DiffResponse_DIFF_NONE,
		"prop")
}

func TestComputedSetUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": computedValue,
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedComputedSetUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedComputedSetAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedComputedSetUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedComputedSetIntUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeInt}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"prop": []interface{}{42},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedComputedSetIntUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeInt}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"prop": []interface{}{42},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestNestedComputedSetIntAdd(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeInt}},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{computedValue},
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": A,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedSetUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": computedValue,
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedSetNestedUpdate(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": computedValue}},
		},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0].nest": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedSetNestedUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true, ForceNew: true},
					},
				},
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": computedValue}},
		},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0].nest": UR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestComputedSetNestedIgnore(t *testing.T) {
	for _, ignore := range []string{"prop[0]", "prop"} {
		diffTest(t,
			map[string]*schema.Schema{
				"prop": {
					Type: schema.TypeSet,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nest": {Type: schema.TypeString, Required: true},
						},
					},
				},
				"outp": {Type: schema.TypeString, Computed: true},
			},
			map[string]*SchemaInfo{},
			map[string]interface{}{
				"prop": []interface{}{map[string]interface{}{"nest": computedValue}},
			},
			map[string]interface{}{
				"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
				"outp": "bar",
			},
			map[string]DiffKind{},
			pulumirpc.DiffResponse_DIFF_NONE,
			ignore)
	}
}

func TestRawElementNames(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"variables": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": map[string]interface{}{
				"variables": map[string]interface{}{
					"DYNAMODB_ROUTE_TABLE_NAME": "foo",
				},
			},
		},
		map[string]interface{}{
			"prop": map[string]interface{}{
				"variables": map[string]interface{}{
					"DYNAMODB_ROUTE_TABLE_NAME": "bar",
				},
			},
		},
		map[string]DiffKind{
			"prop.variables.DYNAMODB_ROUTE_TABLE_NAME": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

// SETS AND LISTS WITH MULTIPLE ITEMS

func TestCollectionsWithMultipleItems(t *testing.T) {
	testCases := []struct {
		name                   string
		state                  []interface{}
		input                  []interface{}
		expectedDiffForSet     map[string]DiffKind
		expectedChangesForSet  pulumirpc.DiffResponse_DiffChanges
		expectedDiffForList    map[string]DiffKind
		expectedChangesForList pulumirpc.DiffResponse_DiffChanges
	}{
		{
			"NoChanges",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{},
			pulumirpc.DiffResponse_DIFF_NONE,
			map[string]DiffKind{},
			pulumirpc.DiffResponse_DIFF_NONE,
		},
		{
			"Reordered",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"tineke", "burgundy", "ruby"},
			map[string]DiffKind{},
			pulumirpc.DiffResponse_DIFF_NONE,
			map[string]DiffKind{
				"prop[0]": UR,
				"prop[1]": UR,
				"prop[2]": UR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"RemoveFirst",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"ruby", "tineke"},
			map[string]DiffKind{
				"prop[0]": DR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[0]": UR,
				"prop[1]": UR,
				"prop[2]": DR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"RemoveMiddle",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "tineke"},
			map[string]DiffKind{
				"prop[1]": DR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[1]": UR,
				"prop[2]": DR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"RemoveLast",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "ruby"},
			map[string]DiffKind{
				"prop[2]": DR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[2]": DR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"AddFirst",
			[]interface{}{"ruby", "tineke"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[0]": AR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[0]": UR,
				"prop[1]": UR,
				"prop[2]": AR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"AddMiddle",
			[]interface{}{"burgundy", "tineke"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[1]": AR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[1]": UR,
				"prop[2]": AR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"AddLast",
			[]interface{}{"burgundy", "ruby"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[2]": AR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[2]": AR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"UpdateFirst",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"robusta", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[0]": UR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[0]": UR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"UpdateMiddle",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "robusta", "tineke"},
			map[string]DiffKind{
				"prop[1]": UR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[1]": UR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
		{
			"UpdateLast",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "ruby", "robusta"},
			map[string]DiffKind{
				"prop[2]": UR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
			map[string]DiffKind{
				"prop[2]": UR,
			},
			pulumirpc.DiffResponse_DIFF_SOME,
		},
	}

	runTestCase := func(t *testing.T, name string, typ schema.ValueType, inputs, state []interface{},
		expected map[string]DiffKind, expectedChanges pulumirpc.DiffResponse_DiffChanges) {
		t.Run(name, func(t *testing.T) {
			diffTest(t,
				map[string]*schema.Schema{
					"prop": {
						Type:     typ,
						Elem:     &schema.Schema{Type: schema.TypeString},
						ForceNew: true,
					},
					"outp": {Type: schema.TypeString, Computed: true},
				},
				map[string]*SchemaInfo{},
				map[string]interface{}{
					"prop": inputs, // inputs
				},
				map[string]interface{}{
					"prop": state, // state
					"outp": "bar",
				},
				expected,
				expectedChanges,
			)
		})
	}

	t.Run("Set", func(t *testing.T) {
		for _, tc := range testCases {
			runTestCase(t, tc.name, schema.TypeSet, tc.input, tc.state, tc.expectedDiffForSet, tc.expectedChangesForSet)
		}
	})

	t.Run("List", func(t *testing.T) {
		for _, tc := range testCases {
			runTestCase(t, tc.name, schema.TypeList, tc.input, tc.state, tc.expectedDiffForList, tc.expectedChangesForList)
		}
	})
}

func TestSetNestedAddReplace(t *testing.T) {
	diffTest(t,
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		map[string]*SchemaInfo{},
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "baz"}},
		},
		map[string]interface{}{
			"prop": nil,
			"outp": "bar",
		},
		map[string]DiffKind{
			"prop[0].nest": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListNestedAddReplace(t *testing.T) {
	diffTest(t,
		// tfSchema
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		// info
		map[string]*SchemaInfo{},
		// inputs
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
		},
		// state
		map[string]interface{}{
			"prop": nil,
			"outp": "bar",
		},
		// expected
		map[string]DiffKind{
			"prop[0].nest": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListNestedUpdate(t *testing.T) {
	diffTest(t,
		// tfSchema
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		// info
		map[string]*SchemaInfo{},
		// inputs
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "foo"}},
		},
		// state
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "bar"}},
			"outp": "bar",
		},
		// expected
		map[string]DiffKind{
			"prop[0].nest": U,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListNestedDeleteReplace(t *testing.T) {
	diffTest(t,
		// tfSchema
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		// info
		map[string]*SchemaInfo{},
		// inputs
		map[string]interface{}{},
		// state
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "bar"}},
			"outp": "bar",
		},
		// expected
		map[string]DiffKind{
			"prop[0].nest": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestSetNestedDeleteReplace(t *testing.T) {
	diffTest(t,
		// tfSchema
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		// info
		map[string]*SchemaInfo{},
		// inputs
		map[string]interface{}{},
		// state
		map[string]interface{}{
			"prop": []interface{}{map[string]interface{}{"nest": "bar"}},
			"outp": "bar",
		},
		// expected
		map[string]DiffKind{
			"prop[0].nest": DR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

func TestListNestedAddMaxItemsOne(t *testing.T) {
	diffTest(t,
		// tfSchema
		map[string]*schema.Schema{
			"prop": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nest": {Type: schema.TypeString, Required: true},
					},
				},
				MaxItems: 1,
				ForceNew: true,
			},
			"outp": {Type: schema.TypeString, Computed: true},
		},
		// info
		map[string]*SchemaInfo{},
		// inputs
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
		},
		// state
		map[string]interface{}{
			"outp": "bar",
		},
		// expected
		map[string]DiffKind{
			"prop.nest": AR,
		},
		pulumirpc.DiffResponse_DIFF_SOME)
}

type diffTestCase struct {
	resourceSchema              map[string]*schema.Schema
	resourceFields              map[string]*SchemaInfo
	state                       resource.PropertyMap
	inputs                      resource.PropertyMap
	expected                    map[string]*pulumirpc.PropertyDiff
	expectedDiffChanges         pulumirpc.DiffResponse_DiffChanges
	ignoreChanges               []string
	XSkipDetailedDiffForChanges bool
}

func diffTest2(t *testing.T, tc diffTestCase) {
	ctx := context.Background()
	res := &schema.Resource{
		Schema: tc.resourceSchema,
	}
	provider := shimv1.NewProvider(&schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"p_resource": res,
		},
	})
	state, err := plugin.MarshalProperties(tc.state, plugin.MarshalOptions{})
	require.NoError(t, err)

	inputs, err := plugin.MarshalProperties(tc.inputs, plugin.MarshalOptions{})
	require.NoError(t, err)

	p := Provider{
		tf: provider,
		info: ProviderInfo{
			XSkipDetailedDiffForChanges: tc.XSkipDetailedDiffForChanges,
			Resources: map[string]*ResourceInfo{
				"p_resource": {
					Tok:    "pkg:index:PResource",
					Fields: tc.resourceFields,
				},
			},
		},
	}

	p.initResourceMaps()

	resp, err := p.Diff(ctx, &pulumirpc.DiffRequest{
		Id:            "myResource",
		Urn:           "urn:pulumi:test::test::pkg:index:PResource::n1",
		Olds:          state,
		News:          inputs,
		IgnoreChanges: tc.ignoreChanges,
	})
	require.NoError(t, err)

	assert.Equal(t, tc.expectedDiffChanges, resp.Changes)
	require.Equal(t, tc.expected, resp.DetailedDiff)
}

func TestChangingMaxItems1FilterProperty(t *testing.T) {
	schema := map[string]*schema.Schema{
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
	diffTest2(t, diffTestCase{
		XSkipDetailedDiffForChanges: true,
		resourceSchema:              schema,
		state: resource.PropertyMap{
			"rules": resource.NewArrayProperty(
				[]resource.PropertyValue{
					resource.NewObjectProperty(
						resource.PropertyMap{
							"filter": resource.NewNullProperty(),
						},
					),
				},
			),
		},
		inputs: resource.PropertyMap{
			"rules": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"filter": resource.NewObjectProperty(
						resource.PropertyMap{
							"__defaults": resource.NewArrayProperty(
								[]resource.PropertyValue{},
							),
						},
					),
				}),
			}),
		},
		expectedDiffChanges: pulumirpc.DiffResponse_DIFF_SOME,
		expected: map[string]*pulumirpc.PropertyDiff{
			"rules[0].filter": {
				Kind: pulumirpc.PropertyDiff_UPDATE,
			},
		},
	})
}
