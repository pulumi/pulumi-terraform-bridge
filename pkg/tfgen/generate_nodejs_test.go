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

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/stretchr/testify/assert"
)

type typeTest struct {
	schema         *schema.Schema
	info           *tfbridge.SchemaInfo
	expectedOutput string
	expectedInput  string
}

var tsTypeTests = []typeTest{
	{
		// Bool Schema
		schema:         &schema.Schema{Type: schema.TypeBool},
		expectedOutput: "boolean",
		expectedInput:  "pulumi.Input<boolean>",
	},
	{
		// Int Schema
		schema:         &schema.Schema{Type: schema.TypeInt},
		expectedOutput: "number",
		expectedInput:  "pulumi.Input<number>",
	},
	{
		// Float Schema
		schema:         &schema.Schema{Type: schema.TypeFloat},
		expectedOutput: "number",
		expectedInput:  "pulumi.Input<number>",
	},
	{
		// String Schema
		schema:         &schema.Schema{Type: schema.TypeString},
		expectedOutput: "string",
		expectedInput:  "pulumi.Input<string>",
	},
	{
		// Basic Set Schema
		schema: &schema.Schema{
			Type: schema.TypeSet,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
		expectedOutput: "string[]",
		expectedInput:  "pulumi.Input<pulumi.Input<string>[]>",
	},
	{
		// Basic List Schema
		schema: &schema.Schema{
			Type: schema.TypeList,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
		expectedOutput: "string[]",
		expectedInput:  "pulumi.Input<pulumi.Input<string>[]>",
	},
	{
		// Basic Map Schema
		schema: &schema.Schema{
			Type: schema.TypeMap,
			Elem: &schema.Schema{Type: schema.TypeString},
		},
		expectedOutput: "{[key: string]: string}",
		expectedInput:  "pulumi.Input<{[key: string]: pulumi.Input<string>}>",
	},
	{
		// Resource Map Schema
		schema: &schema.Schema{
			Type: schema.TypeMap,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"foo": {Type: schema.TypeString},
				},
			},
		},
		expectedOutput: "{ foo: string }",
		expectedInput:  "pulumi.Input<{ foo: pulumi.Input<string> }>",
	},
	{
		// Basic alt types
		info: &tfbridge.SchemaInfo{
			Type:     "string",
			AltTypes: []tokens.Type{"Foo"},
		},
		expectedOutput: "string",
		expectedInput:  "pulumi.Input<string | Foo>",
	},
	{
		// Basic array alt types
		info: &tfbridge.SchemaInfo{
			Type:     "string",
			AltTypes: []tokens.Type{"Foo[]"},
		},
		expectedOutput: "string",
		expectedInput:  "pulumi.Input<string | Foo[]>",
	},
	{
		// Complex array alt types
		info: &tfbridge.SchemaInfo{
			Type:     "string",
			AltTypes: []tokens.Type{"pkg:mod/foo:Foo[]"},
		},
		expectedOutput: "string",
		expectedInput:  "pulumi.Input<string | Foo[]>",
	},
	{
		// Asset
		schema: &schema.Schema{Type: schema.TypeString},
		info: &tfbridge.SchemaInfo{
			Asset: &tfbridge.AssetTranslation{
				Kind: tfbridge.FileAsset,
			},
		},
		expectedOutput: "pulumi.asset.Asset | pulumi.asset.Archive",
		expectedInput:  "pulumi.Input<pulumi.asset.Asset | pulumi.asset.Archive>",
	},
	{
		// Archive
		schema: &schema.Schema{Type: schema.TypeString},
		info: &tfbridge.SchemaInfo{
			Asset: &tfbridge.AssetTranslation{
				Kind: tfbridge.FileArchive,
			},
		},
		expectedOutput: "pulumi.asset.Archive",
		expectedInput:  "pulumi.Input<pulumi.asset.Archive>",
	},
}

func Test_TsTypes(t *testing.T) {
	for _, test := range tsTypeTests {
		v := &variable{
			name:   "foo",
			schema: test.schema,
			info:   test.info,
			opt:    true,
			typ:    makePropertyType(test.schema, test.info, false, parsedDoc{}),
		}

		// Output
		v.out = true
		assert.Equal(t, test.expectedOutput, tsType("", "", v, nil, nil, false, false, false))

		// Input
		v.out = false
		assert.Equal(t, test.expectedInput, tsType("", "", v, nil, nil, false, true, false))
	}
}

func Test_Issue130(t *testing.T) {
	schema := &schema.Schema{
		Type:     schema.TypeList,
		MaxItems: 1,
		Elem:     &schema.Schema{Type: schema.TypeString},
	}

	assert.Equal(t, "string", tsType("", "", &variable{
		name:   "condition",
		schema: schema,
		out:    true,
		typ:    makePropertyType(schema, nil, true, parsedDoc{}),
	}, nil, nil, false, false, false))

	assert.Equal(t, "pulumi.Input<string>", tsType("", "", &variable{
		name:   "condition",
		schema: schema,
		out:    false,
		typ:    makePropertyType(schema, nil, false, parsedDoc{}),
	}, nil, nil, false, true, false))
}

func Test_GatherCustomImports_ComplexArrayAltType(t *testing.T) {
	expected := importMap{
		"./foo": map[string]bool{
			"Foo": true,
		},
	}

	g := &nodeJSGenerator{pkg: "pkg"}
	info := tfbridge.SchemaInfo{
		Type:     "string",
		AltTypes: []tokens.Type{"pkg:mod/foo:Foo[]"},
	}
	actual := make(importMap)

	err := g.gatherCustomImports(newModule("mod"), &info, actual)

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}
