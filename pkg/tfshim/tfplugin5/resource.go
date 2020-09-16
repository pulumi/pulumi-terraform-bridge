package tfplugin5

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-cty/cty"
	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/schema"
)

var _ = shim.Resource((*resource)(nil))
var _ = shim.ResourceMap(resourceMap{})

type resource struct {
	provider *provider

	resourceType  string
	ctyType       cty.Type
	schema        schema.SchemaMap
	schemaVersion int
}

func (r *resource) Schema() shim.SchemaMap {
	return r.schema
}

func (r *resource) SchemaVersion() int {
	return r.schemaVersion
}

//nolint: staticcheck
func (r *resource) Importer() shim.ImportFunc {
	if r.provider == nil {
		return nil
	}
	return r.provider.importResourceState
}

func (r *resource) DeprecationMessage() string {
	return ""
}

func (r *resource) Timeouts() *shim.ResourceTimeout {
	return &shim.ResourceTimeout{}
}

func (r *resource) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
	// Stamp the ID into the object.
	object["id"] = id

	// Return an instance state.
	return &instanceState{
		resourceType: r.resourceType,
		id:           id,
		object:       object,
		meta:         meta,
	}, nil
}

func parseTimeout(timeouts *shim.ResourceTimeout, timeoutsMap map[string]interface{}, key string) error {
	timeoutValue, ok := timeoutsMap[key]
	if !ok {
		return nil
	}
	timeoutString, ok := timeoutValue.(string)
	if !ok {
		return fmt.Errorf("%v timeout must be a string", key)
	}
	duration, err := time.ParseDuration(timeoutString)
	if err != nil {
		return fmt.Errorf("failed to parse %v timeout: %w", key, err)
	}
	switch key {
	case "create":
		timeouts.Create = &duration
	case "update":
		timeouts.Update = &duration
	case "read":
		timeouts.Read = &duration
	case "delete":
		timeouts.Delete = &duration
	case "default":
		timeouts.Default = &duration
	default:
		return fmt.Errorf("%v timeout is unsupported", key)
	}
	return nil
}

func (r *resource) DecodeTimeouts(c shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	config, ok := c.(resourceConfig)
	if !ok {
		return nil, fmt.Errorf("internal error: foreign resource config")
	}

	timeoutsValue, ok := config["timeouts"]
	if !ok {
		return &shim.ResourceTimeout{}, nil
	}
	timeoutsMap, ok := timeoutsValue.(map[string]interface{})
	if !ok {
		return &shim.ResourceTimeout{}, nil
	}

	timeouts := &shim.ResourceTimeout{}
	for _, key := range []string{"create", "update", "read", "delete", "default"} {
		if err := parseTimeout(timeouts, timeoutsMap, key); err != nil {
			return nil, err
		}
	}
	return timeouts, nil
}

type resourceMap map[string]*resource

func (m resourceMap) Len() int {
	return len(m)
}

func (m resourceMap) Get(key string) shim.Resource {
	r, _ := m.GetOk(key)
	return r
}

func (m resourceMap) GetOk(key string) (shim.Resource, bool) {
	if r, ok := m[key]; ok {
		return r, true
	}
	return nil, false
}

func (m resourceMap) Range(each func(key string, value shim.Resource) bool) {
	for key, value := range m {
		if !each(key, value) {
			return
		}
	}
}

func (m resourceMap) Set(key string, value shim.Resource) {
	m[key] = value.(*resource)
}
