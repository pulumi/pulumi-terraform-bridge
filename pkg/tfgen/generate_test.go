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

package tfgen

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
)

func Test_DeprecationFromTFSchema(t *testing.T) {
	v := &variable{
		name:   "v",
		schema: shimv1.NewSchema(&schema.Schema{Type: schema.TypeString, Deprecated: "This is deprecated"}),
	}

	deprecationMessage := v.deprecationMessage()
	assert.Equal(t, "This is deprecated", deprecationMessage)
}

func Test_ForceNew(t *testing.T) {
	cases := []struct {
		Name           string
		Var            variable
		ShouldForceNew bool
	}{
		{Name: "Pulumi Schema with ForceNew Override ShouldForceNew true",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type: schema.TypeString,
				}),
				info: &tfbridge.SchemaInfo{
					ForceNew: tfbridge.True(),
				},
			},
			ShouldForceNew: true,
		},
		{
			Name: "TF Schema ForceNew ShouldForceNew true",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type:     schema.TypeString,
					ForceNew: true,
				}),
			},
			ShouldForceNew: true,
		},
		{
			Name: "Output Parameter ShouldForceNew false",
			Var: variable{
				out: true,
			},
			ShouldForceNew: false,
		},
		{
			Name: "Input Non ForceNew Parameter ShouldForceNew false",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type: schema.TypeString,
				}),
			},
			ShouldForceNew: false,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			v := &test.Var
			actuallyForcesNew := v.forceNew()
			assert.Equal(t, test.ShouldForceNew, actuallyForcesNew)
		})
	}
}

func Test_GenerateTestDataSchemas(t *testing.T) {
	// This is to assert that all the schemas we save in tf2pulumi/convert/testdata/schemas, match up with the
	// mapping files in tf2pulumi/convert/testdata/mappings. Add in the use of PULUMI_ACCEPT and it means you
	// don't have to manually write schemas, just mappings for tests.

	testDir, err := filepath.Abs(filepath.Join("..", "tf2pulumi", "convert", "testdata"))
	require.NoError(t, err)
	mappingsPath := filepath.Join(testDir, "mappings")
	schemasPath := filepath.Join(testDir, "schemas")
	mapper := &bridgetesting.TestFileMapper{Path: mappingsPath}
	providerInfoSource := il.NewMapperProviderInfoSource(mapper)

	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	})

	// Generate the schemas from the mappings
	infos, err := os.ReadDir(mappingsPath)
	require.NoError(t, err)
	for _, info := range infos {
		// Strip off the .json part to make the package name
		pkg := strings.Replace(info.Name(), filepath.Ext(info.Name()), "", -1)
		provInfo, err := providerInfoSource.GetProviderInfo("", "", pkg, "")
		require.NoError(t, err)

		schema, err := GenerateSchema(*provInfo, nilSink)
		require.NoError(t, err)

		schemaPath := filepath.Join(schemasPath, pkg+".json")
		bridgetesting.AssertEqualsJSONFile(t, schemaPath, schema)
	}
}

func Test_ProviderWithObjectTypesInConfigCanGenerateRenames(t *testing.T) {
	strType := (&shimschema.Schema{Type: shim.TypeString}).Shim()
	objType := (&shimschema.Schema{
		Type:     shim.TypeMap,
		Optional: true,
		Elem: (&shimschema.Resource{
			Schema: shimschema.SchemaMap{
				"foo_bar": strType,
			},
		}).Shim(),
	}).Shim()

	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	})

	r, err := GenerateSchemaWithOptions(GenerateSchemaOptions{
		DiagnosticsSink: nilSink,
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "test",
			P: (&shimschema.Provider{
				ResourcesMap:   shimschema.ResourceMap{},
				DataSourcesMap: shimschema.ResourceMap{},
				Schema: &shimschema.SchemaMap{
					"prop": objType,
				},
			}).Shim(),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "foo_bar", r.Renames.RenamedProperties["test:index/ProviderProp:ProviderProp"]["fooBar"])
}
