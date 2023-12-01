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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"
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
		test := test
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
		t.Run(info.Name(), func(t *testing.T) {
			// Strip off the .json part to make the package name
			pkg := strings.Replace(info.Name(), filepath.Ext(info.Name()), "", -1)
			provInfo, err := providerInfoSource.GetProviderInfo("", "", pkg, "")
			require.NoError(t, err)

			schema, err := GenerateSchema(*provInfo, nilSink)
			require.NoError(t, err)

			schemaPath := filepath.Join(schemasPath, pkg+".json")
			bridgetesting.AssertEqualsJSONFile(t, schemaPath, schema)
		})
	}
}

// Encoding of Elem() and Type() is tricky when recognizing types, cover all the cases here documented in the
// shim.Schema.Elem() docstring.
func Test_makePropertyType(t *testing.T) {

	g := &Generator{}
	path := paths.NewProperyPath(paths.NewConfigPath(), paths.PropertyName{
		Key:  "prop",
		Name: tokens.Name("prop"),
	})

	strType := (&shimschema.Schema{Type: shim.TypeString}).Shim()
	intType := (&shimschema.Schema{Type: shim.TypeInt}).Shim()

	xySchema := (&shimschema.Resource{
		Schema: shimschema.SchemaMap{
			"x": strType,
			"y": intType,
		},
	}).Shim()

	t.Run("String", func(t *testing.T) {
		p := g.makePropertyType(path, "obj", strType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindString), p.kind)
	})

	t.Run("ListString", func(t *testing.T) {
		strListType := (&shimschema.Schema{
			Type: shim.TypeList,
			Elem: strType,
		}).Shim()
		p := g.makePropertyType(path, "obj", strListType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindList), p.kind)
		assert.Equal(t, typeKind(kindString), p.element.kind)
	})

	t.Run("MapString", func(t *testing.T) {
		strMapType := (&shimschema.Schema{
			Type: shim.TypeMap,
			Elem: strType,
		}).Shim()
		p := g.makePropertyType(path, "obj", strMapType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindMap), p.kind)
		assert.Equal(t, typeKind(kindString), p.element.kind)
	})

	t.Run("MapUnknown", func(t *testing.T) {
		unkMapType := (&shimschema.Schema{
			Type: shim.TypeMap,
		}).Shim()
		p := g.makePropertyType(path, "obj", unkMapType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindMap), p.kind)
		assert.Nil(t, p.element)
	})

	t.Run("SingleNestedBlock", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type: shim.TypeMap,
			Elem: xySchema,
		}).Shim()
		p := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindObject), p.kind)
		assert.Equal(t, "config.prop", p.properties[0].parentPath.String())
	})

	t.Run("ListNestedBlock", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type: shim.TypeList,
			Elem: xySchema,
		}).Shim()
		p := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindList), p.kind)
		assert.Equal(t, typeKind(kindObject), p.element.kind)
		assert.Equal(t, "config.prop.$", p.element.properties[0].parentPath.String())
	})

	t.Run("ListNestedBlockMaxItem1", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type:     shim.TypeList,
			Elem:     xySchema,
			MaxItems: 1,
		}).Shim()
		p := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindObject), p.kind)
		assert.Equal(t, "config.prop", p.properties[0].parentPath.String())
	})

	t.Run("SetNestedBlock", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type: shim.TypeSet,
			Elem: xySchema,
		}).Shim()
		p := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindSet), p.kind)
		assert.Equal(t, typeKind(kindObject), p.element.kind)
		assert.Equal(t, "config.prop.$", p.element.properties[0].parentPath.String())
	})

	t.Run("SetNestedBlockMaxItem1", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type:     shim.TypeSet,
			Elem:     xySchema,
			MaxItems: 1,
		}).Shim()
		p := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		assert.Equal(t, typeKind(kindObject), p.kind)
		assert.Equal(t, "config.prop", p.properties[0].parentPath.String())
	})
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

func generateNestedSchema(t *testing.T, f func(*tfbridge.ResourceInfo)) pschema.PackageSpec {
	strType := (&shimschema.Schema{Type: shim.TypeString}).Shim()
	nestedObj := (&shimschema.Schema{
		Type:     shim.TypeMap,
		Optional: true,
		Elem: (&shimschema.Resource{
			Schema: shimschema.SchemaMap{
				"fizz_buzz": strType,
			},
		}).Shim(),
	}).Shim()
	objType := (&shimschema.Schema{
		Type:     shim.TypeMap,
		Optional: true,
		Elem: (&shimschema.Resource{
			Schema: shimschema.SchemaMap{
				"foo_bar": strType,
				"nested":  nestedObj,
			},
		}).Shim(),
	}).Shim()

	p := (&shimschema.Provider{
		ResourcesMap: shimschema.ResourceMap{
			"test_res": (&shimschema.Resource{
				Schema: shimschema.SchemaMap{
					"obj": objType,
				},
			}).Shim(),
		},
	}).Shim()

	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	})

	res := &tfbridge.ResourceInfo{
		Tok: "test:index:Bar",
	}
	if f != nil {
		f(res)
	}

	r, err := GenerateSchemaWithOptions(GenerateSchemaOptions{
		DiagnosticsSink: nilSink,
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "test",
			P:    p,
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res": res,
			},
		},
	})
	require.NoError(t, err)
	return r.PackageSpec
}

func Test_ProviderWithOmittedTypes(t *testing.T) {
	t.Parallel()

	t.Run("no-omit", func(t *testing.T) {
		t.Parallel()
		spec := generateNestedSchema(t, nil)
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Resources["test:index:Bar"].InputProperties, 1)
		assert.Len(t, spec.Types, 2)
	})

	t.Run("omit-top-level-prop", func(t *testing.T) {
		t.Parallel()
		spec := generateNestedSchema(t, func(info *tfbridge.ResourceInfo) {
			info.Fields = map[string]*tfbridge.SchemaInfo{
				"obj": {Omit: true},
			}
		})
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Resources["test:index:Bar"].InputProperties, 0)
		assert.Len(t, spec.Types, 0)
	})

	t.Run("omit-nested-prop", func(t *testing.T) {
		t.Parallel()
		spec := generateNestedSchema(t, func(info *tfbridge.ResourceInfo) {
			info.Fields = map[string]*tfbridge.SchemaInfo{
				"obj": {
					Elem: &tfbridge.SchemaInfo{
						Fields: map[string]*tfbridge.SchemaInfo{
							"nested": {Omit: true},
						},
					},
				},
			}
		})
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Resources["test:index:Bar"].InputProperties, 1)
		assert.Len(t, spec.Types, 1)
	})

}

func Test_ProviderWithMovedTypes(t *testing.T) {
	t.Parallel()

	t.Run("none", func(t *testing.T) {
		t.Parallel()
		spec := generateNestedSchema(t, nil)
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Resources["test:index:Bar"].InputProperties, 1)
		assert.Len(t, spec.Types, 2)
		assert.Contains(t, spec.Types, "test:index/BarObj:BarObj")
		assert.Contains(t, spec.Types, "test:index/BarObjNested:BarObjNested")
	})

	t.Run("top-level", func(t *testing.T) {
		t.Parallel()
		spec := generateNestedSchema(t, func(info *tfbridge.ResourceInfo) {
			info.Fields = map[string]*tfbridge.SchemaInfo{
				"obj": {Type: "test:moved:Top"},
			}
		})
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Types, 2)
		if assert.Contains(t, spec.Types, "test:moved:Top") {
			assert.Contains(t, spec.Types, "test:moved:TopNested")
		}
	})

	t.Run("nested-prop", func(t *testing.T) {
		t.Parallel()
		spec := generateNestedSchema(t, func(info *tfbridge.ResourceInfo) {
			info.Fields = map[string]*tfbridge.SchemaInfo{
				"obj": {
					Elem: &tfbridge.SchemaInfo{
						Fields: map[string]*tfbridge.SchemaInfo{
							"nested": {Type: "test:moved:Nested"},
						},
					},
				},
			}
		})
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Types, 2)
		assert.Contains(t, spec.Types, "test:index/BarObj:BarObj")
		assert.Contains(t, spec.Types, "test:moved:Nested")
	})

	t.Run("conflict", func(t *testing.T) {
		t.Parallel()
		t.Fatalf("TODO Test that the provider errors when multiple types are directed to the same token.")
	})

}

func TestModulePlacementForType(t *testing.T) {
	t.Parallel()

	type testCase struct {
		pkg    tokens.Package
		path   paths.TypePath
		expect tokens.Module
	}

	prop := paths.PropertyName{Key: "prop", Name: "prop"}

	testCases := []testCase{
		// Resource in top-level module mymod is placed to myprov:mymod.
		{
			"myprov",
			paths.NewProperyPath(paths.NewResourcePath(
				"myprov_myres",
				"myprov:mymod:MyRes",
				false, /*isProvider*/
			).Inputs(), prop),
			"myprov:mymod",
		},
		// Resource in second-level module mymod is placed to parent myprov:mymod.
		{
			"myprov",
			paths.NewProperyPath(paths.NewResourcePath(
				"myprov_myres",
				"myprov:mymod/mysubmod:MyRes",
				false, /*isProvider*/
			).Inputs(), prop),
			"myprov:mymod",
		},
		// Resource in third-level module mymod is placed to parent myprov:mymod/mysubmod.
		{
			"myprov",
			paths.NewProperyPath(paths.NewResourcePath(
				"myprov_myres",
				"myprov:mymod/mysubmod/mysubsubmod:MyRes",
				false, /*isProvider*/
			).Inputs(), prop),
			"myprov:mymod/mysubmod",
		},
		// Datasource in top-level module mymod is placed to myprov:mymod.
		{
			"myprov",
			paths.NewProperyPath(paths.NewDataSourcePath(
				"myprov_myds",
				"myprov:mymod:MyFn",
			).Args(), prop),
			"myprov:mymod",
		},
		// Datasource in second-level module mymod is placed to parent myprov:mymod.
		{
			"myprov",
			paths.NewProperyPath(paths.NewDataSourcePath(
				"myprov_myds",
				"myprov:mymod/mysubmod:MyFn",
			).Args(), prop),
			"myprov:mymod",
		},
		// Datasource in third-level module mymod is placed to parent myprov:mymod/mysubmod.
		{
			"myprov",
			paths.NewProperyPath(paths.NewDataSourcePath(
				"myprov_ds",
				"myprov:mymod/mysubmod/mysubsubmod:MyFn",
			).Args(), prop),
			"myprov:mymod/mysubmod",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("testCase-%d", i), func(t *testing.T) {
			t.Parallel()
			mod := modulePlacementForType(tc.pkg, tc.path)
			assert.Equal(t, tc.expect, mod)
		})
	}

}

func TestWithoutPackageName(t *testing.T) {
	assert.Equal(t, "http", withoutPackageName("http", "http"))
	assert.Equal(t, "s3_bucket", withoutPackageName("aws", "aws_s3_bucket"))
}
