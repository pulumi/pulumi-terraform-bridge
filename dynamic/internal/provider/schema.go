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

package provider

import (
	otshim "github.com/opentofu/opentofu/shim"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var _ = shim.SchemaMap(object{})

type object struct {

	// schema is assumed to be valid.
	//
	// It is assumed that `schema.Block.Attributes` does not have any conflicts with
	// `schema.Block.BlockTypes`.
	schema otshim.Schema
}

func (o object) Len() int {
	return len(o.schema.Block.Attributes) + len(o.schema.Block.BlockTypes)
}

func (o object) Get(key string) shim.Schema {
	s, ok := o.GetOk(key)
	contract.Assertf(ok, "Could not find object %s in object", key)
	return s
}

func (o object) GetOk(key string) (shim.Schema, bool) {
	if a, ok := o.schema.Block.Attributes[key]; ok {
		return attribute{a}, true
	}
	if n, ok := o.schema.Block.BlockTypes[key]; ok {
		return block{n}, true
	}

	return nil, false
}

func (o object) Range(each func(key string, value shim.Schema) bool) {
	for key, a := range o.schema.Block.Attributes {
		if !each(key, attribute{a}) {
			return
		}
	}

	for key, n := range o.schema.Block.BlockTypes {
		if !each(key, block{n}) {
			return
		}
	}
}

func (o object) Set(key string, value shim.Schema) { panic("CANNOT MUTATE AN OBJECT") }

func (o object) Delete(key string) { panic("CANNOT MUTATE AN OBJECT") }

// TODO: Do we need to do this, since it should have already been called when the provider
// was loaded remotely.
func (o object) Validate() error { return o.schema.Block.InternalValidate() }
