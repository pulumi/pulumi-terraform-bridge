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
	Refresh(t string, s InstanceState) (InstanceState, error)

	ReadDataDiff(t string, c ResourceConfig) (InstanceDiff, error)
	ReadDataApply(t string, d InstanceDiff) (InstanceState, error)

	Meta() interface{}
	Stop() error

	InitLogging()
	NewDestroyDiff() InstanceDiff
	NewResourceConfig(object map[string]interface{}) ResourceConfig
	IsSet(v interface{}) ([]interface{}, bool)
}
