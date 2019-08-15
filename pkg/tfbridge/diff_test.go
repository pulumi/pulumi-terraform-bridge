package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/pulumi/pulumi/pkg/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"github.com/stretchr/testify/assert"
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

func diffTest(t *testing.T, sch map[string]*schema.Schema, info map[string]*SchemaInfo,
	inputs, state map[string]interface{}, expected map[string]DiffKind, ignoreChanges ...string) {

	inputsMap := resource.NewPropertyMapFromMap(inputs)
	stateMap := resource.NewPropertyMapFromMap(state)

	// Fake up a TF resource and a TF provider.
	res := &schema.Resource{
		Schema: sch,
		CustomizeDiff: func(d *schema.ResourceDiff, _ interface{}) error {
			return d.SetNewComputed("outp")
		},
	}
	provider := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"resource": res,
		},
	}

	// Convert the inputs and state to TF config and resource attributes.
	attrs, meta, err := MakeTerraformAttributes(res, stateMap, sch, info, resource.PropertyMap{}, false)
	assert.NoError(t, err)

	config, err := MakeTerraformConfig(nil, inputsMap, sch, info, resource.PropertyMap{}, false)
	assert.NoError(t, err)

	tfDiff, err := provider.SimpleDiff(&terraform.InstanceInfo{Type: "resource"},
		&terraform.InstanceState{ID: "id", Attributes: attrs, Meta: meta}, config)
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
