package schema

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

var _ = shim.ActionMap(ActionMap{})

type Action struct {
	Schema   shim.SchemaMap
	Metadata string
	Invoke   func(ctx context.Context, ps resource.PropertyMap) (resource.PropertyMap, error)
}

func (r *Action) Shim() shim.Action {
	return ActionShim{V: r}
}

type ActionShim struct {
	V *Action
	internalinter.Internal
}

func (r ActionShim) Schema() shim.SchemaMap {
	return r.V.Schema
}

func (r ActionShim) Metadata() string {
	return r.V.Metadata
}

func (r ActionShim) Invoke(ctx context.Context, ps resource.PropertyMap) (resource.PropertyMap, error) {
	return r.V.Invoke(ctx, ps)
}

type ActionMap map[string]shim.Action

func (m ActionMap) Len() int {
	return len(m)
}

func (m ActionMap) Get(key string) shim.Action {
	return m[key]
}

func (m ActionMap) GetOk(key string) (shim.Action, bool) {
	r, ok := m[key]
	return r, ok
}

func (m ActionMap) Range(each func(key string, value shim.Action) bool) {
	for key, value := range m {
		if !each(key, value) {
			return
		}
	}
}

func (m ActionMap) Set(key string, value shim.Action) {
	m[key] = value
}
