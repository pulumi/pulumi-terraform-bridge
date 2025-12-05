package sdkv2

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestImpliedType(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, res schema.Resource) {
		impliedType := res.CoreConfigSchema().ImpliedType()

		marshalledRes := info.MarshalResourceShim(NewResource(&res))

		unmarshalledRes := marshalledRes.Unmarshal()
		unmarshalledImpliedType := shimschema.ImpliedType(unmarshalledRes.Schema(), false)

		if !unmarshalledImpliedType.Equals(htype2ctype(impliedType)) {
			t.Errorf("expected implied type to be: \n%v\ngot: \n%v", impliedType.GoString(), unmarshalledImpliedType.GoString())
		}
	}

	t.Run("primitive types", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeString},
				"y": {Type: schema.TypeInt, Optional: true},
				"z": {Type: schema.TypeBool},
				"w": {Type: schema.TypeFloat},
			},
		}

		check(t, res)
	})

	t.Run("collection attributes", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
				"y": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
				"z": {Type: schema.TypeMap, Elem: &schema.Schema{Type: schema.TypeString}},
			},
		}

		check(t, res)
	})

	t.Run("collection blocks", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeList, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"y": {Type: schema.TypeString},
				}}},
				"z": {Type: schema.TypeSet, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"y": {Type: schema.TypeString},
				}}},
			},
		}

		check(t, res)
	})

	t.Run("nested blocks", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeList, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"y": {Type: schema.TypeSet, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"z": {Type: schema.TypeString},
					}}},
				}}},
			},
		}

		check(t, res)
	})
}

func TestImpliedTypeTimeouts(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, res schema.Resource) {
		impliedType := res.CoreConfigSchema().ImpliedType()

		marshalledRes := info.MarshalResourceShim(NewResource(&res))

		unmarshalledRes := marshalledRes.Unmarshal()
		unmarshalledImpliedType := shimschema.ImpliedType(unmarshalledRes.Schema(), true)

		if !unmarshalledImpliedType.Equals(htype2ctype(impliedType)) {
			t.Errorf("expected implied type to be: \n%v\ngot: \n%v", impliedType.GoString(), unmarshalledImpliedType.GoString())
		}
	}

	t.Run("primitive types", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Timeouts: &schema.ResourceTimeout{
				Create:  schema.DefaultTimeout(10 * time.Second),
				Read:    schema.DefaultTimeout(10 * time.Second),
				Update:  schema.DefaultTimeout(10 * time.Second),
				Delete:  schema.DefaultTimeout(10 * time.Second),
				Default: schema.DefaultTimeout(10 * time.Second),
			},
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeString},
				"y": {Type: schema.TypeInt, Optional: true},
				"z": {Type: schema.TypeBool},
				"w": {Type: schema.TypeFloat},
			},
		}

		check(t, res)
	})

	t.Run("collection attributes", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Timeouts: &schema.ResourceTimeout{
				Create:  schema.DefaultTimeout(10 * time.Second),
				Read:    schema.DefaultTimeout(10 * time.Second),
				Update:  schema.DefaultTimeout(10 * time.Second),
				Delete:  schema.DefaultTimeout(10 * time.Second),
				Default: schema.DefaultTimeout(10 * time.Second),
			},
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeList, Elem: &schema.Schema{Type: schema.TypeString}},
				"y": {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
				"z": {Type: schema.TypeMap, Elem: &schema.Schema{Type: schema.TypeString}},
			},
		}

		check(t, res)
	})

	t.Run("collection blocks", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Timeouts: &schema.ResourceTimeout{
				Create:  schema.DefaultTimeout(10 * time.Second),
				Read:    schema.DefaultTimeout(10 * time.Second),
				Update:  schema.DefaultTimeout(10 * time.Second),
				Delete:  schema.DefaultTimeout(10 * time.Second),
				Default: schema.DefaultTimeout(10 * time.Second),
			},
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeList, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"y": {Type: schema.TypeString},
				}}},
				"z": {Type: schema.TypeSet, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"y": {Type: schema.TypeString},
				}}},
			},
		}

		check(t, res)
	})

	t.Run("nested blocks", func(t *testing.T) {
		t.Parallel()

		res := schema.Resource{
			Timeouts: &schema.ResourceTimeout{
				Create:  schema.DefaultTimeout(10 * time.Second),
				Read:    schema.DefaultTimeout(10 * time.Second),
				Update:  schema.DefaultTimeout(10 * time.Second),
				Delete:  schema.DefaultTimeout(10 * time.Second),
				Default: schema.DefaultTimeout(10 * time.Second),
			},
			Schema: map[string]*schema.Schema{
				"x": {Type: schema.TypeList, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"y": {Type: schema.TypeSet, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
						"z": {Type: schema.TypeString},
					}}},
				}}},
			},
		}

		check(t, res)
	})
}
