package sdkv2

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var (
	_ = shim.Schema(v2Schema{})
	_ = shim.SchemaWithNewSet(v2Schema{})
	_ = shim.SchemaWithUnknownCollectionSupported(v2Schema{})
	_ = shim.SchemaMap(v2SchemaMap{})
	_ = shim.SchemaWithWriteOnly(v2Schema{})
	_ = shim.SchemaWithSetElementHash(v2Schema{})
)

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
	return withPatchedDefaults(s.tf).Default
}

func (s v2Schema) DefaultFunc() shim.SchemaDefaultFunc {
	return shim.SchemaDefaultFunc(withPatchedDefaults(s.tf).DefaultFunc)
}

func (s v2Schema) DefaultValue() (interface{}, error) {
	return withPatchedDefaults(s.tf).DefaultValue()
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

func (s v2Schema) ExactlyOneOf() []string {
	return s.tf.ExactlyOneOf
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

func (s v2Schema) WriteOnly() bool {
	return s.tf.WriteOnly
}

// SetElement expects a set element without any unknown values.
// The value passed here can contain golang types or a plugin-sdk schema.Set.
func (s v2Schema) SetElement(v interface{}) (interface{}, error) {
	// The plugin-sdk does some pre-processing on the set element values before
	// calling the hash function on them. Some examples of that are:
	// - dropping unknowns
	// - defaulting null values (false for bools, 0 for ints, "" for strings)
	// Instead of trying to reverse that, we'll just call the plugin-sdk's
	// code for accessing set elements which handles this pre-processing.
	// This ensures that the hash functions receives the values it expects.
	// See [pkg/tests/tfcheck/tf_test.go:TestTFSetHashNil] for an example of
	// this behaviour in TF.
	sch := map[string]*schema.Schema{"e": s.tf}
	setWriter := &schema.MapFieldWriter{Schema: sch}
	err := setWriter.WriteField([]string{"e"}, []interface{}{v})
	if err != nil {
		return nil, err
	}
	reader := &schema.MapFieldReader{
		Schema: sch,
		Map:    schema.BasicMapReader(setWriter.Map()),
	}

	field, err := reader.ReadField([]string{"e"})
	if err != nil {
		return nil, err
	}
	return field.Value.(*schema.Set).List()[0], nil
}

func (s v2Schema) SetHash(v interface{}) int {
	//nolint:lll
	// adapted from https://github.com/pulumi/terraform-plugin-sdk/blob/4f60ee4e2975c25b88b392e69c87551bb0e26dfc/helper/schema/set.go#L220
	code := s.tf.ZeroValue().(*schema.Set).F(v)
	if code < 0 {
		return -code
	}
	return code
}

func (s v2Schema) SetElementHash(v interface{}) (int, error) {
	v, err := s.SetElement(v)
	if err != nil {
		return 0, err
	}
	return s.SetHash(v), nil
}

func (s v2Schema) NewSet(v []interface{}) interface{} {
	return schema.NewSet(s.SetHash, v)
}

type v2SchemaMap map[string]*schema.Schema

func NewSchemaMap(m map[string]*schema.Schema) shim.SchemaMap {
	return v2SchemaMap(m)
}

func (m v2SchemaMap) unwrap() map[string]*schema.Schema {
	return m
}

func (m v2SchemaMap) Validate() error {
	return schema.InternalMap(m).InternalValidate(m.unwrap())
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

func (s v2Schema) SupportsUnknownCollections() {}
