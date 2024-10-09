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
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	_ "pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Value represents a single conceptual value, represented in both a [cty.Value] and in a
// [resource.PropertyValue].
type Value struct {
	Type schema.Schema
	Tf   cty.Value
	Pu   resource.PropertyMap
}

func WithValue(t schema.Schema) Value {
	panic("UNIMPLEMENTED")
}
