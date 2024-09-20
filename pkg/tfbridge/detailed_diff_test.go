package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestIsBlock(t *testing.T) {
	tests := []struct {
		name string
		s    shim.Schema
		want bool
	}{
		{
			name: "block",
			s:    shimv2.NewSchema(&schema.Schema{Elem: &schema.Resource{}}),
			want: true,
		},
		{
			name: "schema",
			s:    shimv2.NewSchema(&schema.Schema{Elem: &schema.Schema{}}),
			want: false,
		},
		{
			name: "nil",
			s:    shimv2.NewSchema(&schema.Schema{Elem: nil}),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBlock(tt.s); got != tt.want {
				t.Errorf("isBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func computeSchemas(sch map[string]*schema.Schema) (map[string]*info.Schema, shim.SchemaMap) {
	tfp := &schema.Provider{ResourcesMap: map[string]*schema.Resource{
		"prov_res": {Schema: sch},
	}}
	shimProvider := shimv2.NewProvider(tfp)

	provider := ProviderInfo{
		P:                              shimProvider,
		Name:                           "prov",
		Version:                        "0.0.1",
		MetadataInfo:                   &MetadataInfo{},
		EnableZeroDefaultSchemaVersion: true,
	}
	makeToken := func(module, name string) (string, error) {
		return tokens.MakeStandard("prov")(module, name)
	}
	provider.MustComputeTokens(tokens.SingleModule("prov", "index", makeToken))

	return provider.Resources["prov_res"].Fields, provider.P.ResourcesMap().Get("prov_res").Schema()
}

func TestMakePropDiff(t *testing.T) {
	tests := []struct {
		name  string
		old   resource.PropertyValue
		new   resource.PropertyValue
		oldOk bool
		newOk bool
		want  *pulumirpc.PropertyDiff
	}{
		{
			name:  "unchanged non-nil",
			old:   resource.NewStringProperty("same"),
			new:   resource.NewStringProperty("same"),
			oldOk: true,
			newOk: true,
			want:  nil,
		},
		{
			name:  "unchanged nil",
			old:   resource.NewNullProperty(),
			new:   resource.NewNullProperty(),
			oldOk: true,
			newOk: true,
			want:  nil,
		},
		{
			name:  "unchanged not present",
			old:   resource.NewNullProperty(),
			new:   resource.NewNullProperty(),
			oldOk: false,
			newOk: false,
			want:  nil,
		},
		{
			name:  "added",
			old:   resource.NewNullProperty(),
			new:   resource.NewStringProperty("new"),
			oldOk: false,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_ADD},
		},
		{
			name:  "deleted",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewNullProperty(),
			oldOk: true,
			newOk: false,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_DELETE},
		},
		{
			name:  "changed non-nil",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewStringProperty("new"),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
		{
			name:  "changed from nil",
			old:   resource.NewNullProperty(),
			new:   resource.NewStringProperty("new"),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
		{
			name:  "changed to nil",
			old:   resource.NewStringProperty("old"),
			new:   resource.NewNullProperty(),
			oldOk: true,
			newOk: true,
			want:  &pulumirpc.PropertyDiff{Kind: pulumirpc.PropertyDiff_UPDATE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeTopPropDiff(tt.old, tt.new, tt.oldOk, tt.newOk)
			if got == nil && tt.want == nil {
				return
			}
			if got == nil || tt.want == nil {
				t.Errorf("makeTopPropDiff() = %v, want %v", got, tt.want)
				return
			}
			if got.Kind != tt.want.Kind {
				t.Errorf("makeTopPropDiff() = %v, want %v", got.String(), tt.want.String())
			}
		})
	}
}

func TestBasicDetailedDiff(t *testing.T) {
	A := map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_ADD},
	}
	U := map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_UPDATE},
	}
	D := map[string]*pulumirpc.PropertyDiff{
		"foo": {Kind: pulumirpc.PropertyDiff_DELETE},
	}

	runTest := func(
		t *testing.T,
		old, new resource.PropertyMap,
		tfs shim.SchemaMap,
		ps map[string]*SchemaInfo,
		want map[string]*pulumirpc.PropertyDiff,
	) {
		got := makePulumiDetailedDiffV2(context.Background(), tfs, ps, old, new)

		if len(got) != len(want) {
			t.Fatalf("got %d diffs, want %d", len(got), len(want))
		}

		for k, v := range got {
			wantV, ok := want[k]
			if !ok {
				t.Fatalf("unexpected diff %s", k)
			}
			if v.Kind != wantV.Kind {
				t.Errorf("got diff %s = %v, want %v", k, v.Kind, wantV.Kind)
			}
		}
	}

	for _, tt := range []struct {
		name       string
		emptyValue interface{}
		value1     interface{}
		value2     interface{}
		tfs        schema.Schema
	}{
		{
			name:       "string",
			emptyValue: "",
			value1:     "foo",
			value2:     "bar",
			tfs:        schema.Schema{Type: schema.TypeString},
		},
		{
			name:       "int",
			emptyValue: nil,
			value1:     42,
			value2:     43,
			tfs:        schema.Schema{Type: schema.TypeInt},
		},
		{
			name:       "bool",
			emptyValue: nil,
			value1:     true,
			value2:     false,
			tfs:        schema.Schema{Type: schema.TypeBool},
		},
		{
			name:       "float",
			emptyValue: nil,
			value1:     42.0,
			value2:     43.0,
			tfs:        schema.Schema{Type: schema.TypeFloat},
		},
		{
			name: "list",
			tfs: schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{"foo"},
			value2:     []interface{}{"bar"},
		},
		{
			name: "map",
			tfs: schema.Schema{
				Type: schema.TypeMap,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			emptyValue: map[string]interface{}{},
			value1:     map[string]interface{}{"foo": "bar"},
			value2:     map[string]interface{}{"foo": "baz"},
		},
		{
			name: "set",
			tfs: schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Schema{Type: schema.TypeString},
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{"foo"},
			value2:     []interface{}{"bar"},
		},
		{
			name: "list block",
			tfs: schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {Type: schema.TypeString},
					},
				},
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
		},
		{
			name: "max items one list block",
			tfs: schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {Type: schema.TypeString},
					},
				},
				MaxItems: 1,
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
		},
		{
			name: "set block",
			tfs: schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {Type: schema.TypeString},
					},
				},
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
		},
		{
			name: "max items one set block",
			tfs: schema.Schema{
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {Type: schema.TypeString},
					},
				},
				MaxItems: 1,
			},
			emptyValue: []interface{}{},
			value1:     []interface{}{map[string]interface{}{"foo": "bar"}},
			value2:     []interface{}{map[string]interface{}{"foo": "baz"}},
		},
		// TODO: object tests
		// TODO: list tests
	} {
		t.Run(tt.name, func(t *testing.T) {
			sdkv2Schema := map[string]*schema.Schema{
				"foo": &tt.tfs,
			}
			ps, tfs := computeSchemas(sdkv2Schema)
			propertyMapNil := resource.NewPropertyMapFromMap(
				map[string]interface{}{},
			)
			propertyMapEmpty := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.emptyValue,
				},
			)
			propertyMapValue1 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value1,
				},
			)
			propertyMapValue2 := resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"foo": tt.value2,
				},
			)

			t.Run("unchanged", func(t *testing.T) {
				runTest(t, propertyMapValue1, propertyMapValue1, tfs, ps, nil)
			})

			t.Run("changed non-empty", func(t *testing.T) {
				runTest(t, propertyMapValue1, propertyMapValue2, tfs, ps, U)
			})

			t.Run("added", func(t *testing.T) {
				runTest(t, propertyMapNil, propertyMapValue1, tfs, ps, A)
			})

			t.Run("deleted", func(t *testing.T) {
				runTest(t, propertyMapValue1, propertyMapNil, tfs, ps, D)
			})

			if tt.emptyValue != nil {
				t.Run("changed from empty", func(t *testing.T) {
					runTest(t, propertyMapEmpty, propertyMapValue1, tfs, ps, U)
				})

				t.Run("changed to empty", func(t *testing.T) {
					runTest(t, propertyMapValue1, propertyMapEmpty, tfs, ps, U)
				})

				t.Run("unchanged empty", func(t *testing.T) {
					runTest(t, propertyMapEmpty, propertyMapEmpty, tfs, ps, nil)
				})

				t.Run("deleted empty", func(t *testing.T) {
					runTest(t, propertyMapEmpty, propertyMapNil, tfs, ps, D)
				})

				t.Run("added empty", func(t *testing.T) {
					runTest(t, propertyMapNil, propertyMapEmpty, tfs, ps, A)
				})
			}
		})
	}
}
