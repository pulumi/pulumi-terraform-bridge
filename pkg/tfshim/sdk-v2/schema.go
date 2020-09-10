package sdkv2

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
)

var _ = shim.Schema(v2Schema{})
var _ = shim.SchemaMap(v2SchemaMap{})

// UnknownVariableValue is the sentinal defined in github.com/hashicorp/terraform/configs/hcl2shim,
// representing a variable whose value is not known at some particular time. The value is duplicated here in
// order to prevent an additional dependency - it is unlikely to ever change upstream since that would break
// rather a lot of things.
const UnknownVariableValue = "74D93920-ED26-11E3-AC10-0800200C9A66"

type v2Schema struct {
	tf *schema.Schema
}

func NewSchema(s *schema.Schema) shim.Schema {
	return v2Schema{s}
}

func (s v2Schema) Type() shim.ValueType {
	switch s.tf.Type {
	case schema.TypeBool:
		return shim.TypeBool
	case schema.TypeInt:
		return shim.TypeInt
	case schema.TypeFloat:
		return shim.TypeFloat
	case schema.TypeString:
		return shim.TypeString
	case schema.TypeList:
		return shim.TypeList
	case schema.TypeMap:
		return shim.TypeMap
	case schema.TypeSet:
		return shim.TypeSet
	default:
		return shim.TypeInvalid
	}
}

func (s v2Schema) Optional() bool {
	return s.tf.Optional
}

func (s v2Schema) Required() bool {
	return s.tf.Required
}

func (s v2Schema) Default() interface{} {
	return s.tf.Default
}

func (s v2Schema) DefaultFunc() shim.SchemaDefaultFunc {
	return shim.SchemaDefaultFunc(s.tf.DefaultFunc)
}

func (s v2Schema) DefaultValue() (interface{}, error) {
	return s.tf.DefaultValue()
}

func (s v2Schema) Description() string {
	return s.tf.Description
}

func (s v2Schema) Computed() bool {
	return s.tf.Computed
}

func (s v2Schema) ForceNew() bool {
	return s.tf.ForceNew
}

func (s v2Schema) StateFunc() shim.SchemaStateFunc {
	return shim.SchemaStateFunc(s.tf.StateFunc)
}

func (s v2Schema) Elem() interface{} {
	switch e := s.tf.Elem.(type) {
	case *schema.Resource:
		return v2Resource{e}
	case *schema.Schema:
		return v2Schema{e}
	default:
		return nil
	}
}

func (s v2Schema) MaxItems() int {
	return s.tf.MaxItems
}

func (s v2Schema) MinItems() int {
	return s.tf.MinItems
}

func (s v2Schema) ConflictsWith() []string {
	return s.tf.ConflictsWith
}

func (s v2Schema) Removed() string {
	return ""
}

func (s v2Schema) Deprecated() string {
	return s.tf.Deprecated
}

func (s v2Schema) Sensitive() bool {
	return s.tf.Sensitive
}

// makeUnknownElement creates an unknown value to be used as an element of a list or set using the given
// element schema to guide the shape of the value.
func makeUnknownElement(elem interface{}) interface{} {
	// If we have no element schema, just return a simple unknown.
	if elem == nil {
		return UnknownVariableValue
	}

	switch e := elem.(type) {
	case *schema.Schema:
		// If the element uses a normal schema, defer to UnknownValue.
		return v2Schema{e}.UnknownValue()
	case *schema.Resource:
		// If the element uses a resource schema, fill in unknown values for any required properties.
		res := make(map[string]interface{})
		for k, v := range e.Schema {
			if v.Required {
				res[k] = v2Schema{v}.UnknownValue()
			}
		}
		return res
	default:
		return UnknownVariableValue
	}
}

func (s v2Schema) UnknownValue() interface{} {
	switch s.tf.Type {
	case schema.TypeList, schema.TypeSet:
		// TF does not accept unknown lists or sets. Instead, it accepts lists or sets of unknowns.
		count := 1
		if s.tf.MinItems > 0 {
			count = s.tf.MinItems
		}
		arr := make([]interface{}, count)
		for i := range arr {
			arr[i] = makeUnknownElement(s.tf.Elem)
		}
		return arr
	default:
		return UnknownVariableValue
	}
}

func (s v2Schema) SetElement(v interface{}) (interface{}, error) {
	raw := map[string]interface{}{"e": []interface{}{v}}
	reader := &schema.ConfigFieldReader{
		Config: &terraform.ResourceConfig{Raw: raw, Config: raw},
		Schema: map[string]*schema.Schema{"e": s.tf},
	}
	field, err := reader.ReadField([]string{"e"})
	if err != nil {
		return nil, err
	}
	return field.Value.(*schema.Set).List()[0], nil
}

func (s v2Schema) SetHash(v interface{}) int {
	code := s.tf.ZeroValue().(*schema.Set).F(v)
	if code < 0 {
		return -code
	}
	return code
}

type v2SchemaMap map[string]*schema.Schema

func NewSchemaMap(m map[string]*schema.Schema) shim.SchemaMap {
	return v2SchemaMap(m)
}

func (m v2SchemaMap) Len() int {
	return len(m)
}

func (m v2SchemaMap) Get(key string) shim.Schema {
	s, _ := m.GetOk(key)
	return s
}

func (m v2SchemaMap) GetOk(key string) (shim.Schema, bool) {
	if s, ok := m[key]; ok {
		return v2Schema{s}, true
	}
	return nil, false
}

func (m v2SchemaMap) Range(each func(key string, value shim.Schema) bool) {
	for key, value := range m {
		if !each(key, v2Schema{value}) {
			return
		}
	}
}

func (m v2SchemaMap) Set(key string, value shim.Schema) {
	m[key] = value.(v2Schema).tf
}

func (m v2SchemaMap) Delete(key string) {
	delete(m, key)
}
