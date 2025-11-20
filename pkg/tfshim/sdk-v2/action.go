package sdkv2

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var (
	_ = shim.Action(v2Action{})
	_ = shim.ActionMap(v2ActionMap{})
)

type v2Action struct {
	internalinter.Internal
}

func (a v2Action) Schema() shim.SchemaMap {
	contract.Failf("v2Action does not support Schema")
	return nil
}

func (a v2Action) Metadata() string {
	contract.Failf("v2Action does not support Metadata")
	return ""
}

func (a v2Action) Invoke(
	ctx context.Context,
	inputs resource.PropertyMap,
) (resource.PropertyMap, error) {
	contract.Failf("v2Action does not support Invoke")
	return nil, nil
}

type v2ActionMap map[string]any

func (m v2ActionMap) Len() int {
	return 0
}

func (m v2ActionMap) Get(key string) shim.Action {
	return nil
}

func (m v2ActionMap) GetOk(key string) (shim.Action, bool) {
	return nil, false
}

func (m v2ActionMap) Range(each func(key string, value shim.Action) bool) {
}

func (m v2ActionMap) Set(key string, value shim.Action) {
	contract.Failf("cannot set on v2ActionMap")
}
