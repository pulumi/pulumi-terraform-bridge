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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type schemaOnlyResource struct {
	tf *tfsdk.Schema
}

var _ shim.Resource = (*schemaOnlyResource)(nil)

func (r *schemaOnlyResource) Schema() shim.SchemaMap {
	return newSchemaMap(r.tf)
}

func (*schemaOnlyResource) SchemaVersion() int         { panic("TODO") }
func (*schemaOnlyResource) DeprecationMessage() string { panic("TODO") }

func (*schemaOnlyResource) Importer() shim.ImportFunc {
	panic("schemaOnlyResource does not implement runtime operation ImporterFunc")
}

func (*schemaOnlyResource) Timeouts() *shim.ResourceTimeout {
	panic("schemaOnlyResource does not implement runtime operation Timeouts")
}

func (*schemaOnlyResource) InstanceState(id string, object,
	meta map[string]interface{}) (shim.InstanceState, error) {
	panic("schemaOnlyResource does not implement runtime operation InstanceState")
}

func (*schemaOnlyResource) DecodeTimeouts(
	config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("schemaOnlyResource does not implement runtime operation DecodeTimeouts")
}
