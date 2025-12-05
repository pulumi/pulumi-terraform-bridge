package schema

import (
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var _ = shim.ResourceMap(ResourceMap{})

type Resource struct {
	Schema             shim.SchemaMap
	SchemaVersion      int
	Importer           shim.ImportFunc
	DeprecationMessage string
	Timeouts           *shim.ResourceTimeout
	SchemaType         valueshim.Type
}

func (r *Resource) Shim() shim.Resource {
	return ResourceShim{V: r}
}

var (
	_ shim.Resource                    = ResourceShim{}
	_ shim.ResourceWithHasDynamicTypes = ResourceShim{}
)

type ResourceShim struct {
	V *Resource
	internalinter.Internal
}

func (r ResourceShim) Schema() shim.SchemaMap {
	return r.V.Schema
}

func (r ResourceShim) SchemaVersion() int {
	return r.V.SchemaVersion
}

func (r ResourceShim) Importer() shim.ImportFunc {
	return r.V.Importer
}

func (r ResourceShim) DeprecationMessage() string {
	return r.V.DeprecationMessage
}

func (r ResourceShim) SchemaType() valueshim.Type {
	return r.V.SchemaType
}

func (r ResourceShim) Timeouts() *shim.ResourceTimeout {
	return r.V.Timeouts
}

func (r ResourceShim) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
	return nil, fmt.Errorf("mock schema does not support instance states")
}

func (r ResourceShim) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	return nil, fmt.Errorf("mock schema does not support resource timeout decoding")
}

func (r ResourceShim) HasDynamicTypes() bool {
	hasDynamicTypes := false
	r.V.Schema.Range(func(key string, value shim.Schema) bool {
		schemaWithHasDynamicTypes, ok := value.(shim.SchemaWithHasDynamicTypes)
		contract.Assertf(ok, "Schema must implement SchemaWithHasDynamicTypes")
		if schemaWithHasDynamicTypes.HasDynamicTypes() {
			hasDynamicTypes = true
			return false
		}
		return true
	})
	return hasDynamicTypes
}

type ResourceMap map[string]shim.Resource

func (m ResourceMap) Len() int {
	return len(m)
}

func (m ResourceMap) Get(key string) shim.Resource {
	return m[key]
}

func (m ResourceMap) GetOk(key string) (shim.Resource, bool) {
	r, ok := m[key]
	return r, ok
}

func (m ResourceMap) Range(each func(key string, value shim.Resource) bool) {
	for key, value := range m {
		if !each(key, value) {
			return
		}
	}
}

func (m ResourceMap) Set(key string, value shim.Resource) {
	m[key] = value
}
