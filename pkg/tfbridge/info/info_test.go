package info

import (
	"testing"

	"github.com/hexops/autogold/v2"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	schemashim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestMarshallableResourceShimSchemaType(t *testing.T) {
	t.Parallel()

	t.Run("plain attributes", func(t *testing.T) {
		t.Parallel()
		resource := schemashim.Resource{
			Schema: schemashim.SchemaMap{
				"name": (&schemashim.Schema{
					Type: shim.TypeString,
				}).Shim(),
			},
		}

		marshalledResource := MarshalResourceShim(resource.Shim())
		unmarshalledResource := marshalledResource.Unmarshal()

		autogold.Expect(`cty.Object(map[string]cty.Type{"id":cty.String, "name":cty.String, "timeouts":cty.Object(map[string]cty.Type{"create":cty.String, "default":cty.String, "delete":cty.String, "read":cty.String, "update":cty.String})})`).Equal(t, unmarshalledResource.SchemaType().GoString())
	})

	t.Run("collections", func(t *testing.T) {
		t.Parallel()
		resource := schemashim.Resource{
			Schema: schemashim.SchemaMap{
				"x": (&schemashim.Schema{
					Type: shim.TypeList,
					Elem: (&schemashim.Schema{
						Type: shim.TypeString,
					}).Shim(),
				}).Shim(),
				"y": (&schemashim.Schema{
					Type: shim.TypeSet,
					Elem: (&schemashim.Schema{
						Type: shim.TypeString,
					}).Shim(),
				}).Shim(),
				"z": (&schemashim.Schema{
					Type: shim.TypeMap,
					Elem: (&schemashim.Schema{
						Type: shim.TypeString,
					}).Shim(),
				}).Shim(),
			},
		}

		marshalledResource := MarshalResourceShim(resource.Shim())
		unmarshalledResource := marshalledResource.Unmarshal()

		autogold.Expect(`cty.Object(map[string]cty.Type{"id":cty.String, "timeouts":cty.Object(map[string]cty.Type{"create":cty.String, "default":cty.String, "delete":cty.String, "read":cty.String, "update":cty.String}), "x":cty.List(cty.String), "y":cty.Set(cty.String), "z":cty.Map(cty.String)})`).Equal(t, unmarshalledResource.SchemaType().GoString())
	})

	t.Run("blocks", func(t *testing.T) {
		t.Parallel()
		resource := schemashim.Resource{
			Schema: schemashim.SchemaMap{
				"x": (&schemashim.Schema{
					Type: shim.TypeList,
					Elem: (&schemashim.Resource{
						Schema: schemashim.SchemaMap{
							"y": (&schemashim.Schema{
								Type: shim.TypeString,
							}).Shim(),
						},
					}).Shim(),
				}).Shim(),
			},
		}

		marshalledResource := MarshalResourceShim(resource.Shim())
		unmarshalledResource := marshalledResource.Unmarshal()

		autogold.Expect(`cty.Object(map[string]cty.Type{"id":cty.String, "timeouts":cty.Object(map[string]cty.Type{"create":cty.String, "default":cty.String, "delete":cty.String, "read":cty.String, "update":cty.String}), "x":cty.List(cty.Object(map[string]cty.Type{"y":cty.String}))})`).Equal(t, unmarshalledResource.SchemaType().GoString())
	})
}
