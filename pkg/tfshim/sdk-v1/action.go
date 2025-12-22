package sdkv1

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var (
	_ = shim.Action(v1Action{})
	_ = shim.ActionMap(v1ActionMap{})
)

type v1Action struct {
	internalinter.Internal
}

func (a v1Action) Schema() shim.SchemaMap {
	contract.Failf("v1Action does not support Schema")
	return nil
}

func (a v1Action) Metadata() string {
	contract.Failf("v1Action does not support Metadata")
	return ""
}

func (a v1Action) Invoke(
	ctx context.Context,
	inputs resource.PropertyMap,
) (resource.PropertyMap, error) {
	contract.Failf("v1Action does not support Invoke")
	return nil, nil
}

type v1ActionMap map[string]any

func (m v1ActionMap) Len() int {
	return 0
}

func (m v1ActionMap) Get(key string) shim.Action {
	return nil
}

func (m v1ActionMap) GetOk(key string) (shim.Action, bool) {
	return nil, false
}

func (m v1ActionMap) Range(each func(key string, value shim.Action) bool) {
}

func (m v1ActionMap) Set(key string, value shim.Action) {
	contract.Failf("cannot set on v1ActionMap")
}
