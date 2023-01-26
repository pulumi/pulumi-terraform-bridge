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
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type schemaOnlyDataSource struct {
	tf pfutils.Schema
}

var _ shim.Resource = (*schemaOnlyDataSource)(nil)

func (r *schemaOnlyDataSource) Schema() shim.SchemaMap {
	return newSchemaMap(r.tf)
}

func (*schemaOnlyDataSource) SchemaVersion() int { panic("TODO") }

func (r *schemaOnlyDataSource) DeprecationMessage() string {
	return r.tf.DeprecationMessage()
}

func (*schemaOnlyDataSource) Importer() shim.ImportFunc {
	panic("schemaOnlyDataSource does not implement runtime operation ImporterFunc")
}

func (*schemaOnlyDataSource) Timeouts() *shim.ResourceTimeout {
	panic("schemaOnlyDataSource does not implement runtime operation Timeouts")
}

func (*schemaOnlyDataSource) InstanceState(id string, object,
	meta map[string]interface{}) (shim.InstanceState, error) {
	panic("schemaOnlyDataSource does not implement runtime operation InstanceState")
}

func (*schemaOnlyDataSource) DecodeTimeouts(
	config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("schemaOnlyDataSource does not implement runtime operation DecodeTimeouts")
}
