package shim

import (
	"context"
	"fmt"
	"time"
)

type ResourceConfig interface {
	IsSet(k string) bool
}

type InstanceState interface {
	Type() string

	// Return the resource identifier.
	//
	// If the ID is unknown, as would be the case when previewing a Create, return an empty
	// string (zero value).
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
	HasNoChanges() bool
	ProposedState(res Resource, priorState InstanceState) (InstanceState, error)
	Destroy() bool
	RequiresNew() bool
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
	TypeDynamic
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
	case TypeDynamic:
		return "Dynamic"
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

	// s.Elem() may return a nil, a Schema value, or a Resource value [1].
	//
	// Case 1: s represents a compound type (s.Type() is one of TypeList, TypeSet or TypeMap), and s.Elem()
	// represents the element of this type as a Schema value. That is, if s ~ List[String] then s.Elem() ~ String.
	//
	// Case 2: s represents a single-nested Terraform block. Logically this is like s having an anonymous object
	// type such as s ~ {"x": Int, "y": String}. In this case s.Type() == TypeMap and s.Elem() is a Resource value.
	// This s.Elem() value is not a real Resource and only implements the Schema field to enable inspecting
	// s.Elem().Schema() to find out the names ("x", "y") and types (Int, String) of the block properties. SDKv2
	// providers cannot represent single-nested blocks; this case is only used for Plugin Framework providers. SDKv2
	// providers use a convention to declare a List-nested block with MaxItems=1 to model object types. Per [2]
	// SDKv2 providers reinterpret case 2 as a string-string map for backwards compatibility.
	//
	// Case 3: s represents a list or set-nested Terraform block. That is, s ~ List[{"x": Int, "y": String}]. In
	// this case s.Type() is one of TypeList, TypeSet, and s.Elem() is a Resource that encodes the object type
	// similarly to Case 2.
	//
	// Case 4: s.Elem() is nil and s.Type() is a scalar type (none of TypeList, TypeSet, TypeMap).
	//
	// Case 5: s.Elem() is nil but s.Type() is one of TypeList, TypeSet, TypeMap. The element type is unknown.
	//
	// This encoding cannot support map-nested blocks but it does not need to as those are not expressible in TF.
	//
	// A test suite [3] is provided to explore how Plugin Framework constructs map to Schema.
	//
	// A test suite [4] is provided to explore how SDKv2 constructs map to Schema.
	// [1]: https://github.com/hashicorp/terraform-plugin-sdk/blob/main/helper/schema/schema.go#L231
	// [2]: https://github.com/hashicorp/terraform-plugin-sdk/blob/main/helper/schema/core_schema_test.go#L220
	// [3]: https://github.com/pulumi/pulumi-terraform-bridge/blob/master/pf/tests/schemashim_test.go#L34
	// [4]: https://github.com/pulumi/pulumi-terraform-bridge/blob/master/pkg/tfshim/sdk-v2/shim_test.go#L29
	Elem() interface{}

	MaxItems() int
	MinItems() int
	ConflictsWith() []string
	ExactlyOneOf() []string
	Deprecated() string
	Removed() string
	Sensitive() bool

	SetElement(config interface{}) (interface{}, error)
	SetHash(v interface{}) int
}

type SchemaMap interface {
	Len() int
	Get(key string) Schema
	GetOk(key string) (Schema, bool)
	Range(each func(key string, value Schema) bool)

	Validate() error
}

type ImportFunc func(t, id string, meta interface{}) ([]InstanceState, error)

type TimeoutKey string

const (
	TimeoutCreate  TimeoutKey = "create"
	TimeoutRead    TimeoutKey = "read"
	TimeoutUpdate  TimeoutKey = "update"
	TimeoutDelete  TimeoutKey = "delete"
	TimeoutDefault TimeoutKey = "default"
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

type ResourceMapWithClone interface {
	ResourceMap
	Clone(oldKey, newKey string) error
}

func CloneResource(rm ResourceMap, oldKey string, newKey string) error {
	switch rm := rm.(type) {
	case ResourceMapWithClone:
		return rm.Clone(oldKey, newKey)
	default:
		v, ok := rm.GetOk(oldKey)
		if !ok {
			return fmt.Errorf("key not found: %q", oldKey)
		}
		rm.Set(newKey, v)
		return nil
	}
}

type Provider interface {
	Schema() SchemaMap
	ResourcesMap() ResourceMap
	DataSourcesMap() ResourceMap

	InternalValidate() error
	Validate(ctx context.Context, c ResourceConfig) ([]string, []error)
	ValidateResource(ctx context.Context, t string, c ResourceConfig) ([]string, []error)
	ValidateDataSource(ctx context.Context, t string, c ResourceConfig) ([]string, []error)

	Configure(ctx context.Context, c ResourceConfig) error

	Diff(
		ctx context.Context,
		t string,
		s InstanceState,
		c ResourceConfig,
		opts DiffOptions,
	) (InstanceDiff, error)

	Apply(ctx context.Context, t string, s InstanceState, d InstanceDiff) (InstanceState, error)

	Refresh(
		ctx context.Context, t string, s InstanceState, c ResourceConfig,
	) (InstanceState, error)

	ReadDataDiff(ctx context.Context, t string, c ResourceConfig) (InstanceDiff, error)
	ReadDataApply(ctx context.Context, t string, d InstanceDiff) (InstanceState, error)

	Meta(ctx context.Context) interface{}
	Stop(ctx context.Context) error

	InitLogging(ctx context.Context)

	// Create a Destroy diff for a resource identified by the TF token t.
	NewDestroyDiff(ctx context.Context, t string, opts TimeoutOptions) InstanceDiff

	NewResourceConfig(ctx context.Context, object map[string]interface{}) ResourceConfig
	NewProviderConfig(ctx context.Context, object map[string]interface{}) ResourceConfig

	// Checks if a value is representing a Set, and unpacks its elements on success.
	IsSet(ctx context.Context, v interface{}) ([]interface{}, bool)

	// SupportsUnknownCollections returns false if the provider needs special handling of unknown collections.
	// False for the sdkv1 provider.
	SupportsUnknownCollections() bool
}

type TimeoutOptions struct {
	ResourceTimeout  *ResourceTimeout // optional
	TimeoutOverrides map[TimeoutKey]time.Duration
}

type DiffOptions struct {
	IgnoreChanges  IgnoreChanges
	TimeoutOptions TimeoutOptions
}

// Supports the ignoreChanges Pulumi option.
//
// The bridge needs to be able to suppress diffs computed by the underlying provider.
//
// For legacy reasons, this is implemented in terms of terraform.InstanceDiff object from
// terraform-plugin-sdk. That is, the function needs to return a set of paths that match the keys of
// InstanceDiff.Attributes, something that is slightly complicated to compute correctly for nested
// properties and sets. Diffs that match the keys from the IgnoreChanges set exactly or by prefix
// (sub-diffs) are ignored.
//
// https://www.pulumi.com/docs/concepts/options/ignorechanges/
type IgnoreChanges = func() map[string]struct{}
