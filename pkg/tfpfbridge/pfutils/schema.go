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

package pfutils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Attr type works around not being able to link to fwschema.Schema from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
type Schema interface {
	Type() attr.Type

	Attrs() map[string]Attr
	Blocks() map[string]Block

	DeprecationMessage() string
	AttributeAtPath(context.Context, path.Path) (Attr, diag.Diagnostics)
}

func FromProviderSchema(x pschema.Schema) Schema {
	panic("TODO")
}

func FromDataSourceSchema(x dschema.Schema) Schema {
	panic("TODO")
}

func FromResourceSchema(x rschema.Schema) Schema {
	//x.Type().TerraformType(context.Context)
	panic("TODO")
}
