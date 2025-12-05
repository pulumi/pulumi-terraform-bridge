package schema

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.SchemaMap(SchemaMap{})

type Schema struct {
	Type          shim.ValueType
	Optional      bool
	Required      bool
	Default       interface{}
	DefaultFunc   shim.SchemaDefaultFunc
	Description   string
	Computed      bool
	ForceNew      bool
	StateFunc     shim.SchemaStateFunc
	Elem          interface{}
	MaxItems      int
	MinItems      int
	ConflictsWith []string
	ExactlyOneOf  []string
	Removed       string
	Deprecated    string
	Sensitive     bool
	WriteOnly     bool
}

func (s *Schema) Shim() shim.Schema {
	return SchemaShim{s, internalinter.Internal{}}
}

var (
	_ shim.Schema                    = SchemaShim{}
	_ shim.SchemaWithHasDynamicTypes = SchemaShim{}
)

//nolint:revive
type SchemaShim struct {
	V *Schema
	internalinter.Internal
}

func (s SchemaShim) Type() shim.ValueType {
	return s.V.Type
}

func (s SchemaShim) Optional() bool {
	return s.V.Optional
}

func (s SchemaShim) Required() bool {
	return s.V.Required
}

func (s SchemaShim) Default() interface{} {
	return s.V.Default
}

func (s SchemaShim) DefaultFunc() shim.SchemaDefaultFunc {
	return s.V.DefaultFunc
}

func (s SchemaShim) DefaultValue() (interface{}, error) {
	if s.V.Default != nil {
		return s.V.Default, nil
	}

	if s.V.DefaultFunc != nil {
		defaultValue, err := s.V.DefaultFunc()
		if err != nil {
			return nil, err
		}
		return defaultValue, nil
	}

	return nil, nil
}

func (s SchemaShim) HasDefault() bool {
	return s.V.Default != nil || s.V.DefaultFunc != nil
}

func (s SchemaShim) Description() string {
	return s.V.Description
}

func (s SchemaShim) Computed() bool {
	return s.V.Computed
}

func (s SchemaShim) ForceNew() bool {
	return s.V.ForceNew
}

func (s SchemaShim) StateFunc() shim.SchemaStateFunc {
	return s.V.StateFunc
}

func (s SchemaShim) Elem() interface{} {
	return s.V.Elem
}

func (s SchemaShim) MaxItems() int {
	return s.V.MaxItems
}

func (s SchemaShim) MinItems() int {
	return s.V.MinItems
}

func (s SchemaShim) ConflictsWith() []string {
	return s.V.ConflictsWith
}

func (s SchemaShim) ExactlyOneOf() []string {
	return s.V.ExactlyOneOf
}

func (s SchemaShim) Removed() string {
	return s.V.Removed
}

func (s SchemaShim) Deprecated() string {
	return s.V.Deprecated
}

func (s SchemaShim) Sensitive() bool {
	return s.V.Sensitive
}

func (s SchemaShim) WriteOnly() bool {
	return s.V.WriteOnly
}

func (s SchemaShim) SetElement(v interface{}) (interface{}, error) {
	return v, nil
}

func (s SchemaShim) SetHash(v interface{}) int {
	return 0
}

func (s SchemaShim) NewSet(v []interface{}) interface{} {
	return schema.NewSet(s.SetHash, v)
}

func (s SchemaShim) SetElementHash(v interface{}) (int, error) {
	v, err := s.SetElement(v)
	if err != nil {
		return 0, err
	}
	return s.SetHash(v), nil
}

// modelled after https://github.com/zclconf/go-cty/blob/da4c600729aefcf628d6b042ee439e6927d1104e/cty/type.go#L86
func (s SchemaShim) HasDynamicTypes() bool {
	if s.Type() == shim.TypeDynamic {
		return true
	}

	if s.Type() == shim.TypeList || s.Type() == shim.TypeSet || s.Type() == shim.TypeMap {
		_, isSchemaElem := s.Elem().(shim.Schema)
		if isSchemaElem {
			schemaElem := s.Elem().(shim.SchemaWithHasDynamicTypes)
			return schemaElem.HasDynamicTypes()
		}

		_, isResElem := s.Elem().(shim.Resource)
		if isResElem {
			resElem := s.Elem().(shim.ResourceWithHasDynamicTypes)
			return resElem.HasDynamicTypes()
		}

		// unknown collection element type - best we can do is dynamic
		return true
	}

	return false
}

//nolint:revive
type SchemaMap map[string]shim.Schema

func (m SchemaMap) Validate() error {
	panic("Validate is not yet implemented")
}

func (m SchemaMap) Len() int {
	return len(m)
}

func (m SchemaMap) Get(key string) shim.Schema {
	return m[key]
}

func (m SchemaMap) GetOk(key string) (shim.Schema, bool) {
	s, ok := m[key]
	return s, ok
}

func (m SchemaMap) Range(each func(key string, value shim.Schema) bool) {
	for key, value := range m {
		if !each(key, value) {
			return
		}
	}
}
