package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	v2Schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"

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
		tfState, err := MakeTerraformState(r, "id", stateMap)
		assert.NoError(t, err)

		config, _, err := MakeTerraformConfig(&Provider{tf: provider}, inputsMap, sch, info)
		assert.NoError(t, err)

		tfDiff, err := provider.Diff("resource", tfState, config)
		assert.NoError(t, err)

		// ProcessIgnoreChanges
		doIgnoreChanges(sch, info, stateMap, inputsMap, ignores, tfDiff)

		// Convert the diff to a detailed diff and check the result.
		diff := makeDetailedDiff(sch, info, stateMap, inputsMap, tfDiff)
		expectedDiff := map[string]*pulumirpc.PropertyDiff{}
		for k, v := range expected {
			expectedDiff[k] = &pulumirpc.PropertyDiff{Kind: v}
		}
		assert.Equal(t, expectedDiff, diff)
	})

	t.Run("NoCustomDiffCausesNoDiff", func(t *testing.T) {
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
		tfState, err := MakeTerraformState(r, "id", stateMap)
		assert.NoError(t, err)

		config, _, err := MakeTerraformConfig(&Provider{tf: provider}, inputsMap, sch, info)
		assert.NoError(t, err)

		tfDiff, err := provider.Diff("resource", tfState, config)
		assert.NoError(t, err)

		// ProcessIgnoreChanges
		doIgnoreChanges(sch, info, stateMap, inputsMap, ignores, tfDiff)

		// Convert the diff to a detailed diff and check the result.
		diff := makeDetailedDiff(sch, info, stateMap, inputsMap, tfDiff)
		expectedDiff := map[string]*pulumirpc.PropertyDiff{}
		for k, v := range expected {
			expectedDiff[k] = &pulumirpc.PropertyDiff{Kind: v}
		}
		assert.Equal(t, expectedDiff, diff)
	})
}

func diffTest(t *testing.T, tfs map[string]*schema.Schema, info map[string]*SchemaInfo,
	inputs, state map[string]interface{}, expected map[string]DiffKind, ignoreChanges ...string) {

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
	tfState, err := MakeTerraformState(r, "id", stateMap)
	assert.NoError(t, err)

	config, _, err := MakeTerraformConfig(&Provider{tf: provider}, inputsMap, sch, info)
	assert.NoError(t, err)

	tfDiff, err := provider.Diff("resource", tfState, config)
	assert.NoError(t, err)

	// ProcessIgnoreChanges
	doIgnoreChanges(sch, info, stateMap, inputsMap, ignoreChanges, tfDiff)

	// Convert the diff to a detailed diff and check the result.
	diff := makeDetailedDiff(sch, info, stateMap, inputsMap, tfDiff)
	expectedDiff := map[string]*pulumirpc.PropertyDiff{}
	for k, v := range expected {
		expectedDiff[k] = &pulumirpc.PropertyDiff{Kind: v}
	}
	assert.Equal(t, expectedDiff, diff)

	// Add an ignoreChanges entry for each path in the expected diff, then re-convert the diff and check the result.
	for k := range expected {
		ignoreChanges = append(ignoreChanges, k)
	}
	doIgnoreChanges(sch, info, stateMap, inputsMap, ignoreChanges, tfDiff)

	diff = makeDetailedDiff(sch, info, stateMap, inputsMap, tfDiff)
	assert.Equal(t, map[string]*pulumirpc.PropertyDiff{}, diff)
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
		map[string]DiffKind{})
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
		map[string]DiffKind{})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
}

// SETS AND LISTS WITH MULTIPLE ITEMS

func TestCollectionsWithMultipleItems(t *testing.T) {
	testCases := []struct {
		name                string
		state               []interface{}
		input               []interface{}
		expectedDiffForSet  map[string]DiffKind
		expectedDiffForList map[string]DiffKind
	}{
		{
			"NoChanges",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{},
			map[string]DiffKind{},
		},
		{
			"Reordered",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"tineke", "burgundy", "ruby"},
			map[string]DiffKind{},
			map[string]DiffKind{
				"prop[0]": UR,
				"prop[1]": UR,
				"prop[2]": UR,
			},
		},
		{
			"RemoveFirst",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"ruby", "tineke"},
			map[string]DiffKind{
				"prop[0]": DR,
			},
			map[string]DiffKind{
				"prop[0]": UR,
				"prop[1]": UR,
				"prop[2]": DR,
			},
		},
		{
			"RemoveMiddle",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "tineke"},
			map[string]DiffKind{
				"prop[1]": DR,
			},
			map[string]DiffKind{
				"prop[1]": UR,
				"prop[2]": DR,
			},
		},
		{
			"RemoveLast",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "ruby"},
			map[string]DiffKind{
				"prop[2]": DR,
			},
			map[string]DiffKind{
				"prop[2]": DR,
			},
		},
		{
			"AddFirst",
			[]interface{}{"ruby", "tineke"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[0]": AR,
			},
			map[string]DiffKind{
				"prop[0]": UR,
				"prop[1]": UR,
				"prop[2]": AR,
			},
		},
		{
			"AddMiddle",
			[]interface{}{"burgundy", "tineke"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[1]": AR,
			},
			map[string]DiffKind{
				"prop[1]": UR,
				"prop[2]": AR,
			},
		},
		{
			"AddLast",
			[]interface{}{"burgundy", "ruby"},
			[]interface{}{"burgundy", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[2]": AR,
			},
			map[string]DiffKind{
				"prop[2]": AR,
			},
		},
		{
			"UpdateFirst",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"robusta", "ruby", "tineke"},
			map[string]DiffKind{
				"prop[0]": UR,
			},
			map[string]DiffKind{
				"prop[0]": UR,
			},
		},
		{
			"UpdateMiddle",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "robusta", "tineke"},
			map[string]DiffKind{
				"prop[1]": UR,
			},
			map[string]DiffKind{
				"prop[1]": UR,
			},
		},
		{
			"UpdateLast",
			[]interface{}{"burgundy", "ruby", "tineke"},
			[]interface{}{"burgundy", "ruby", "robusta"},
			map[string]DiffKind{
				"prop[2]": UR,
			},
			map[string]DiffKind{
				"prop[2]": UR,
			},
		},
	}

	//nolint:lll
	runTestCase := func(t *testing.T, name string, typ schema.ValueType, inputs, state []interface{}, expected map[string]DiffKind) {
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
			)
		})
	}

	t.Run("Set", func(t *testing.T) {
		for _, tc := range testCases {
			runTestCase(t, tc.name, schema.TypeSet, tc.input, tc.state, tc.expectedDiffForSet)
		}
	})

	t.Run("List", func(t *testing.T) {
		for _, tc := range testCases {
			runTestCase(t, tc.name, schema.TypeList, tc.input, tc.state, tc.expectedDiffForList)
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
		})
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
		})
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
		})
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
		})
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
		})
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
		})
}

func TestChangingTagsAll(t *testing.T) {
	stateMap := resource.PropertyMap{
		"tagsall": resource.NewObjectProperty(
			resource.PropertyMap{
				"tag1": resource.NewStringProperty("tag1value"),
				"tag2": resource.NewStringProperty("tag2value"),
			},
		),
	}

	inputsMap := resource.PropertyMap{}

	tfs := map[string]*v2Schema.Schema{
		"tagsall": {
			Type:     v2Schema.TypeMap,
			Computed: true,
			Elem: &v2Schema.Schema{
				Type:     v2Schema.TypeString,
				Computed: true,
			},
		},
	}

	sch := shimv2.NewSchemaMap(tfs)

	res := &v2Schema.Resource{
		Schema: tfs,
		CustomizeDiff: func(_ context.Context, d *v2Schema.ResourceDiff, _ interface{}) error {
			return d.SetNew("tagsall", map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2valueModified",
			})
		},
	}

	provider := shimv2.NewProvider(&v2Schema.Provider{
		ResourcesMap: map[string]*v2Schema.Resource{
			"resource": res,
		},
	})

	r := Resource{
		TF: shimv2.NewResource(res),
		Schema: &ResourceInfo{
			Fields: map[string]*SchemaInfo{
				"tagsall": {
					ComputedInput: true,
				},
			},
		},
	}

	tfState, err := MakeTerraformState(r, "id", stateMap)
	assert.NoError(t, err)

	config, _, err := MakeTerraformConfig(&Provider{tf: provider}, inputsMap, sch, nil)
	assert.NoError(t, err)

	tfDiff, err := provider.Diff("resource", tfState, config)
	assert.NoError(t, err)

	// t.Logf("tfDiff = %v", valast.String(tfDiff)) ==>
	//
	// Attributes: map[string]*terraform.ResourceAttrDiff{"tagsall.tag2": &terraform.ResourceAttrDiff{
	//         Old: "tag2value",
	//         New: "tag2valueModified",
	// }},

	// Convert the diff to a detailed diff and check the result.
	diff := makeDetailedDiff(sch, r.Schema.Fields, stateMap, inputsMap, tfDiff)
	assert.Truef(t, len(diff) == 1, "Expected a non-empty diff")
	assert.Contains(t, diff, forceDiffSomeSymbol)
}
