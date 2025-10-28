// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proto

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	cres "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var (
	_ = shim.ActionMap(actionMap{})
	_ = shim.Action(action{})
)

type actionMap map[string]*tfprotov6.ActionSchema

func (m actionMap) Len() int {
	return len(m)
}

func (m actionMap) Get(key string) shim.Action {
	v, ok := m.GetOk(key)
	contract.Assertf(ok, "unknown key %q", key)
	return v
}

func (m actionMap) GetOk(key string) (shim.Action, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	return newAction(v), true
}

func (m actionMap) Range(each func(key string, value shim.Action) bool) {
	for k, v := range m {
		if !each(k, newAction(v)) {
			return
		}
	}
}

func (m actionMap) Set(key string, value shim.Action) {
	v, ok := value.(action)
	contract.Assertf(ok, "Set must be a %T, found a %T", v, value)
	m[key] = v.r
}

type action struct {
	r *tfprotov6.ActionSchema
	internalinter.Internal
}

func newAction(r *tfprotov6.ActionSchema) *action {
	return &action{r, internalinter.Internal{}}
}

func (r action) Schema() shim.SchemaMap {
	return blockMap{r.r.Schema.Block}
}

func (r action) Metadata() string {
	return ""
}

func (r action) Invoke(ctx context.Context, inputs cres.PropertyMap) (cres.PropertyMap, error) {
	return nil, nil
}
