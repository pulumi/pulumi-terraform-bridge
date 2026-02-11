// Copyright 2016-2022, Pulumi Corporation.
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

package schemashim

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

type schemaOnlyListResource struct {
	tf runtypes.Schema
	internalinter.Internal
}

var _ shim.Resource = (*schemaOnlyListResource)(nil)

func (r *schemaOnlyListResource) SchemaType() valueshim.Type {
	protoSchema, err := r.tf.ResourceProtoSchema(context.Background())
	contract.AssertNoErrorf(err, "ResourceProtoSchema failed")
	return valueshim.FromTType(protoSchema.ValueType())
}

func (r *schemaOnlyListResource) Schema() shim.SchemaMap {
	return r.tf.Shim()
}

func (r *schemaOnlyListResource) SchemaVersion() int {
	panic("list resources do not have schema versions")
}

func (r *schemaOnlyListResource) DeprecationMessage() string {
	return r.tf.DeprecationMessage()
}

func (*schemaOnlyListResource) Importer() shim.ImportFunc {
	panic("schemaOnlyListResource does not implement runtime operation ImporterFunc")
}

func (*schemaOnlyListResource) Timeouts() *shim.ResourceTimeout {
	panic("schemaOnlyListResource does not implement runtime operation Timeouts")
}

func (*schemaOnlyListResource) InstanceState(id string, object,
	meta map[string]interface{},
) (shim.InstanceState, error) {
	panic("schemaOnlyListResource does not implement runtime operation InstanceState")
}

func (*schemaOnlyListResource) DecodeTimeouts(
	config shim.ResourceConfig,
) (*shim.ResourceTimeout, error) {
	panic("schemaOnlyListResource does not implement runtime operation DecodeTimeouts")
}
