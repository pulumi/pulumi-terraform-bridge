// Copyright 2016-2023, Pulumi Corporation.
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

package providerbuilder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type Resource struct {
	Name           string
	ResourceSchema schema.Schema
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, re *resource.MetadataResponse) {
	re.TypeName = req.ProviderTypeName + "_" + r.Name
}

func (r *Resource) Schema(ctx context.Context, _ resource.SchemaRequest, re *resource.SchemaResponse) {
	re.Schema = r.ResourceSchema
}

func (*Resource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {}
func (*Resource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse)       {}
func (*Resource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}
func (*Resource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}

var _ resource.Resource = &Resource{}
