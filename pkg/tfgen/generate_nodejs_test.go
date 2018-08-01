// Copyright 2016-2018, Pulumi Corporation.
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

package tfgen

import (
	"testing"

	"github.com/pulumi/pulumi-terraform/pkg/tfbridge"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/stretchr/testify/assert"
)

// Tests that we convert from CamelCase resource names to pythonic
// snake_case correctly.
func Test_TsType(t *testing.T) {

	// Schema output
	assert.Equal(t, "string", tsType(&variable{
		name: "foo",
		schema: &schema.Schema{
			Type: schema.TypeString,
		},
		out: true,
		opt: true,
	}, false, false))

	// Schema input
	assert.Equal(t, "pulumi.Input<string>", tsType(&variable{
		name: "foo",
		schema: &schema.Schema{
			Type: schema.TypeString,
		},
		out: false,
		opt: true,
	}, false, true))

	// AltTypes output
	assert.Equal(t, "string", tsType(&variable{
		name: "foo",
		info: &tfbridge.SchemaInfo{
			Type:     "string",
			AltTypes: []tokens.Type{"Foo"},
		},
		out: true,
		opt: true,
	}, false, false))

	// AltTypes input
	assert.Equal(t, "pulumi.Input<string | Foo>", tsType(&variable{
		name: "foo",
		info: &tfbridge.SchemaInfo{
			Type:     "string",
			AltTypes: []tokens.Type{"Foo"},
		},
		out: false,
		opt: true,
	}, false, true))

}

func Test_Issue130(t *testing.T) {
	schema := &schema.Schema{
		Type:     schema.TypeList,
		MaxItems: 1,
		Elem:     &schema.Schema{Type: schema.TypeString},
	}

	assert.Equal(t, "string", tsType(&variable{
		name:   "condition",
		schema: schema,
		out:    true,
	}, false, false))

	assert.Equal(t, "pulumi.Input<string>", tsType(&variable{
		name:   "condition",
		schema: schema,
		out:    false,
	}, false, true))
}
