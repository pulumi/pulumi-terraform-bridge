package shim

import (
	"time"
)

type ResourceConfig interface {
	IsSet(k string) bool
}

type InstanceState interface {
	Type() string
	ID() string
	Object(sch SchemaMap) (map[string]interface{}, error)
	Meta() map[string]interface{}
}

type DiffAttrType byte

const (
	DiffAttrUnknown DiffAttrType = iota
	DiffAttrInput
	DiffAttrOutput
)

type ResourceAttrDiff struct {
	Old         string
	New         string
	NewComputed bool
	NewRemoved  bool
	NewExtra    interface{}
	RequiresNew bool
	Sensitive   bool
	Type        DiffAttrType
}

type InstanceDiff interface {
	Attribute(key string) *ResourceAttrDiff
	Attributes() map[string]ResourceAttrDiff
	ProposedState(res Resource, priorState InstanceState) (InstanceState, error)
	Destroy() bool
	RequiresNew() bool

	IgnoreChanges(ignored map[string]bool)

	EncodeTimeouts(timeouts *ResourceTimeout) error
	SetTimeout(timeout float64, timeoutKey string)
}

type ValueType int

const (
	TypeInvalid ValueType = iota
	TypeBool
	TypeInt
	TypeFloat
	TypeString
	TypeList
	TypeMap
	TypeSet
)

func (i ValueType) String() string {
	switch i {
	case TypeBool:
		return "Bool"
	case TypeInt:
		return "Int"
	case TypeFloat:
		return "Float"
	case TypeString:
		return "String"
	case TypeList:
		return "List"
	case TypeMap:
		return "Map"
	case TypeSet:
		return "Set"
	default:
		return ""
	}
}

type SchemaDefaultFunc func() (interface{}, error)

type SchemaStateFunc func(interface{}) string

type Schema interface {
	Type() ValueType
	Optional() bool
	Required() bool
	Default() interface{}
	DefaultFunc() SchemaDefaultFunc
	DefaultValue() (interface{}, error)
	Description() string
	Computed() bool
	ForceNew() bool
	StateFunc() SchemaStateFunc

	// s.Elem() may return a nil, a Schema value, or a Resource value.
	//
	// The design of Elem() follows Terraform Plugin SDK directly. Case analysis:
	//
	// Case 1: s represents a compound type (s.Type() is one of TypeList, TypeSet or TypeMap), and s.Elem()
	// represents the element of this type as a Schema value. That is, if s ~ List[String] then s.Elem() ~ String.
	//
	// Case 2: s represents a single-nested Terraform block. Logically this is like s having an anonymous object
	// type such as s ~ {"x": Int, "y": String}. In this case s.Type() == TypeMap and s.Elem() is a Resource value.
	// This value is not a real Resource and only implements the Schema field to enable inspecting s.Elem().Schema()
	// to find out the names ("x", "y") and types (Int, String) of the block properties.
	//
	// Case 3: s represents a list or set-nested Terraform block. That is, s ~ List[{"x": Int, "y": String}]. In
	// this case s.Type() is one of TypeList, TypeSet, and s.Elem() is a Resource that encodes the object type
	// similarly to Case 2.
	//
	// Case 4: s.Elem() is nil and s.Type() is a scalar type (none of TypeList, TypeSet, TypeMap).
	//
	// Case 5: s.Elem() is nil but s.Type() is one of TypeList, TypeSet, TypeMap. The element type is unknown.
	//
	// This encoding cannot support map-nested blocks or object types as it would introduce confusion with Case 2,
	// because Map[String, {"x": Int}] and {"x": Int} both have s.Type() = TypeMap and s.Elem() being a Resource.
	// Following the Terraform design, only set and list-nested blocks are supported.
	//
	// See also: https://github.com/hashicorp/terraform-plugin-sdk/blob/main/helper/schema/schema.go#L231
	Elem() interface{}

	MaxItems() int
	MinItems() int
	ConflictsWith() []string
	ExactlyOneOf() []string
	Deprecated() string
	Removed() string
	Sensitive() bool

	UnknownValue() interface{}

	SetElement(config interface{}) (interface{}, error)
	SetHash(v interface{}) int
}

type SchemaMap interface {
	Len() int
	Get(key string) Schema
	GetOk(key string) (Schema, bool)
	Range(each func(key string, value Schema) bool)

	Set(key string, value Schema)
	Delete(key string)
}

type ImportFunc func(t, id string, meta interface{}) ([]InstanceState, error)

const (
	TimeoutCreate  = "create"
	TimeoutRead    = "read"
	TimeoutUpdate  = "update"
	TimeoutDelete  = "delete"
	TimeoutDefault = "default"
)

type ResourceTimeout struct {
	Create, Read, Update, Delete, Default *time.Duration
}

type Resource interface {
	Schema() SchemaMap
	SchemaVersion() int
	Importer() ImportFunc
	DeprecationMessage() string
	Timeouts() *ResourceTimeout

	InstanceState(id string, object, meta map[string]interface{}) (InstanceState, error)
	DecodeTimeouts(config ResourceConfig) (*ResourceTimeout, error)
}

type ResourceMap interface {
	Len() int
	Get(key string) Resource
	GetOk(key string) (Resource, bool)
	Range(each func(key string, value Resource) bool)

	Set(key string, value Resource)

	AddAlias(alias, target string)
}

type Provider interface {
	Schema() SchemaMap
	ResourcesMap() ResourceMap
	DataSourcesMap() ResourceMap

	Validate(c ResourceConfig) ([]string, []error)
	ValidateResource(t string, c ResourceConfig) ([]string, []error)
	ValidateDataSource(t string, c ResourceConfig) ([]string, []error)

	Configure(c ResourceConfig) error
	Diff(t string, s InstanceState, c ResourceConfig) (InstanceDiff, error)
	Apply(t string, s InstanceState, d InstanceDiff) (InstanceState, error)
	Refresh(t string, s InstanceState, c ResourceConfig) (InstanceState, error)

	ReadDataDiff(t string, c ResourceConfig) (InstanceDiff, error)
	ReadDataApply(t string, d InstanceDiff) (InstanceState, error)

	Meta() interface{}
	Stop() error

	InitLogging()
	NewDestroyDiff() InstanceDiff
	NewResourceConfig(object map[string]interface{}) ResourceConfig
	IsSet(v interface{}) ([]interface{}, bool)
}
