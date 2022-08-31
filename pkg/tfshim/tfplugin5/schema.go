package tfplugin5

import (
	"github.com/hashicorp/go-cty/cty"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// UnknownVariableValue is the sentinal defined in github.com/hashicorp/terraform/configs/hcl2shim,
// representing a variable whose value is not known at some particular time. The value is duplicated here in
// order to prevent an additional dependency - it is unlikely to ever change upstream since that would break
// rather a lot of things.
const UnknownVariableValue = "74D93920-ED26-11E3-AC10-0800200C9A66"

type attributeSchema struct {
	ctyType     cty.Type
	valueType   shim.ValueType
	optional    bool
	required    bool
	description string
	computed    bool
	forceNew    bool
	elem        interface{}
	maxItems    int
	minItems    int
	deprecated  string
	sensitive   bool
}

func (s *attributeSchema) Type() shim.ValueType {
	return s.valueType
}

func (s *attributeSchema) Optional() bool {
	return s.optional
}

func (s *attributeSchema) Required() bool {
	return s.required
}

func (s *attributeSchema) Default() interface{} {
	return nil
}

func (s *attributeSchema) DefaultFunc() shim.SchemaDefaultFunc {
	return nil
}

func (s *attributeSchema) DefaultValue() (interface{}, error) {
	return nil, nil
}

func (s *attributeSchema) Description() string {
	return s.description
}

func (s *attributeSchema) Computed() bool {
	return s.computed
}

func (s *attributeSchema) ForceNew() bool {
	return s.forceNew
}

func (s *attributeSchema) StateFunc() shim.SchemaStateFunc {
	return nil
}

func (s *attributeSchema) Elem() interface{} {
	return s.elem
}

func (s *attributeSchema) MaxItems() int {
	return s.maxItems
}

func (s *attributeSchema) MinItems() int {
	return s.minItems
}

func (s *attributeSchema) ConflictsWith() []string {
	return nil
}

func (s *attributeSchema) ExactlyOneOf() []string {
	return nil
}

func (s *attributeSchema) AtLeastOneOf() []string {
	return nil
}

func (s *attributeSchema) Removed() string {
	return ""
}

func (s *attributeSchema) Deprecated() string {
	return s.deprecated
}

func (s *attributeSchema) Sensitive() bool {
	return s.sensitive
}

func (s *attributeSchema) UnknownValue() interface{} {
	return UnknownVariableValue
}

func (s *attributeSchema) SetElement(v interface{}) (interface{}, error) {
	val, err := goToCty(v, s.ctyType)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (s *attributeSchema) SetHash(v interface{}) int {
	val, ok := v.(cty.Value)
	contract.Assertf(ok, "internal error: SetHash must be a cty.Value")
	return val.Hash()
}
