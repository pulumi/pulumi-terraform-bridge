package sdkv1

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var (
	_ = shim.Schema(v1Schema{})
	_ = shim.SchemaMap(v1SchemaMap{})
)

// UnknownVariableValue is the sentinal defined in github.com/hashicorp/terraform/configs/hcl2shim,
// representing a variable whose value is not known at some particular time. The value is duplicated here in
// order to prevent an additional dependency - it is unlikely to ever change upstream since that would break
// rather a lot of things.
const UnknownVariableValue = "74D93920-ED26-11E3-AC10-0800200C9A66"

type v1Schema struct {
	tf *schema.Schema
	internalinter.Internal
}

func NewSchema(s *schema.Schema) shim.Schema {
	return v1Schema{tf: s}
}

func (s v1Schema) Type() shim.ValueType {
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

func (s v1Schema) Optional() bool {
	return s.tf.Optional
}

func (s v1Schema) Required() bool {
	return s.tf.Required
}

func (s v1Schema) Default() interface{} {
	return s.tf.Default
}

func (s v1Schema) DefaultFunc() shim.SchemaDefaultFunc {
	return shim.SchemaDefaultFunc(s.tf.DefaultFunc)
}

func (s v1Schema) DefaultValue() (interface{}, error) {
	return s.tf.DefaultValue()
}

func (s v1Schema) HasDefault() bool {
	return s.tf.Default != nil || s.tf.DefaultFunc != nil
}

func (s v1Schema) Description() string {
	return s.tf.Description
}

func (s v1Schema) Computed() bool {
	return s.tf.Computed
}

func (s v1Schema) ForceNew() bool {
	return s.tf.ForceNew
}

func (s v1Schema) StateFunc() shim.SchemaStateFunc {
	return shim.SchemaStateFunc(s.tf.StateFunc)
}

func (s v1Schema) Elem() interface{} {
	switch e := s.tf.Elem.(type) {
	case *schema.Resource:
		return v1Resource{tf: e}
	case *schema.Schema:
		return v1Schema{tf: e}
	default:
		return nil
	}
}

func (s v1Schema) MaxItems() int {
	return s.tf.MaxItems
}

func (s v1Schema) MinItems() int {
	return s.tf.MinItems
}

func (s v1Schema) ConflictsWith() []string {
	return s.tf.ConflictsWith
}

func (s v1Schema) ExactlyOneOf() []string {
	return s.tf.ExactlyOneOf
}

func (s v1Schema) Removed() string {
	return s.tf.Removed
}

func (s v1Schema) Deprecated() string {
	return s.tf.Deprecated
}

func (s v1Schema) Sensitive() bool {
	return s.tf.Sensitive
}

func (s v1Schema) SetElement(v interface{}) (interface{}, error) {
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

func (s v1Schema) SetHash(v interface{}) int {
	code := s.tf.ZeroValue().(*schema.Set).F(v)
	if code < 0 {
		return -code
	}
	return code
}

func (s v1Schema) SetElementHash(v interface{}) (int, error) {
	v, err := s.SetElement(v)
	if err != nil {
		return 0, err
	}
	return s.SetHash(v), nil
}

func (s v1Schema) NewSet(v []interface{}) interface{} {
	return schema.NewSet(s.SetHash, v)
}

func (s v1Schema) WriteOnly() bool {
	return false
}

type v1SchemaMap map[string]*schema.Schema

func NewSchemaMap(m map[string]*schema.Schema) shim.SchemaMap {
	return v1SchemaMap(m)
}

func (m v1SchemaMap) unwrap() map[string]*schema.Schema {
	return m
}

func (m v1SchemaMap) Validate() error {
	return schema.InternalMap(m).InternalValidate(m.unwrap())
}

func (m v1SchemaMap) Len() int {
	return len(m)
}

func (m v1SchemaMap) Get(key string) shim.Schema {
	s, _ := m.GetOk(key)
	return s
}

func (m v1SchemaMap) GetOk(key string) (shim.Schema, bool) {
	if s, ok := m[key]; ok {
		return v1Schema{tf: s}, true
	}
	return nil, false
}

func (m v1SchemaMap) Range(each func(key string, value shim.Schema) bool) {
	for key, value := range m {
		if !each(key, v1Schema{tf: value}) {
			return
		}
	}
}
