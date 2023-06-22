package schema

import (
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// UnknownVariableValue is the sentinal defined in github.com/hashicorp/terraform/configs/hcl2shim,
// representing a variable whose value is not known at some particular time. The value is duplicated here in
// order to prevent an additional dependency - it is unlikely to ever change upstream since that would break
// rather a lot of things.
const UnknownVariableValue = "74D93920-ED26-11E3-AC10-0800200C9A66"

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
}

func (s *Schema) Shim() shim.Schema {
	return SchemaShim{s}
}

//nolint:revive
type SchemaShim struct {
	V *Schema
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

func (s SchemaShim) UnknownValue() interface{} {
	return UnknownVariableValue
}

func (s SchemaShim) SetElement(v interface{}) (interface{}, error) {
	return v, nil
}

func (s SchemaShim) SetHash(v interface{}) int {
	return 0
}

//nolint:revive
type SchemaMap map[string]shim.Schema

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

func (m SchemaMap) Set(key string, value shim.Schema) {
	m[key] = value
}

func (m SchemaMap) Delete(key string) {
	delete(m, key)
}
