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
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
)

func Test_DeprecationMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		variable        *variable
		expectedMessage string
	}{
		{
			name: "From TF Schema",
			variable: &variable{
				schema: shimv1.NewSchema(&schema.Schema{Deprecated: "Terraform says this is deprecated"}),
			},
			expectedMessage: "Terraform says this is deprecated",
		},
		{
			name: "From Pulumi Resources File",
			variable: &variable{
				schema: shimv1.NewSchema(&schema.Schema{Deprecated: "Pulumi says this is deprecated"}),
			},
			expectedMessage: "Pulumi says this is deprecated",
		},
		{
			name: "From Pulumi Resources File Overrides TF Schema",
			variable: &variable{
				info:   &tfbridge.SchemaInfo{DeprecationMessage: "Pulumi says this is deprecated"},
				schema: shimv1.NewSchema(&schema.Schema{Deprecated: "Terraform says this is deprecated"}),
			},
			expectedMessage: "Pulumi says this is deprecated",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deprecationMessage := tc.variable.deprecationMessage()
			assert.Equal(t, tc.expectedMessage, deprecationMessage)
		})
	}
}

func Test_ForceNew(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name           string
		Var            variable
		ShouldForceNew bool
	}{
		{
			Name: "Pulumi Schema with ForceNew Override ShouldForceNew true",
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
	t.Parallel()
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
			provInfo.Repository = "https://github.com/pulumi/pulumi-" + pkg

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
	t.Parallel()

	g := &Generator{}
	path := paths.NewProperyPath(paths.NewConfigPath(), paths.PropertyName{
		Key:  "prop",
		Name: tokens.Name("prop"),
	})

	strType := (&shimschema.Schema{Type: shim.TypeString}).Shim()
	intType := (&shimschema.Schema{Type: shim.TypeInt}).Shim()
	dynamicType := (&shimschema.Schema{Type: shim.TypeDynamic}).Shim()

	xySchema := (&shimschema.Resource{
		Schema: shimschema.SchemaMap{
			"x": strType,
			"y": intType,
		},
	}).Shim()

	t.Run("String", func(t *testing.T) {
		p, err := g.makePropertyType(path, "obj", strType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindString, p.kind)
	})

	t.Run("Dynamic Unimplemented", func(t *testing.T) {
		pt, err := g.makePropertyType(path, "obj", dynamicType, nil, false, entityDocs{})
		assert.NoError(t, err)
		assert.Nilf(t, pt, "Expected nil *propertyType as that models the <any> type")
	})

	t.Run("ListString", func(t *testing.T) {
		strListType := (&shimschema.Schema{
			Type: shim.TypeList,
			Elem: strType,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", strListType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindList, p.kind)
		assert.Equal(t, kindString, p.element.kind)
	})

	t.Run("MapString", func(t *testing.T) {
		strMapType := (&shimschema.Schema{
			Type: shim.TypeMap,
			Elem: strType,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", strMapType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindMap, p.kind)
		assert.Equal(t, kindString, p.element.kind)
	})

	t.Run("MapUnknown", func(t *testing.T) {
		unkMapType := (&shimschema.Schema{
			Type: shim.TypeMap,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", unkMapType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindMap, p.kind)
		assert.Equal(t, kindString, p.element.kind)
	})

	t.Run("SingleNestedBlock", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type: shim.TypeMap,
			Elem: xySchema,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindObject, p.kind)
		assert.Equal(t, "config.prop", p.properties[0].parentPath.String())
	})

	t.Run("ListNestedBlock", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type: shim.TypeList,
			Elem: xySchema,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindList, p.kind)
		assert.Equal(t, kindObject, p.element.kind)
		assert.Equal(t, "config.prop.$", p.element.properties[0].parentPath.String())
	})

	t.Run("ListNestedBlockMaxItem1", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type:     shim.TypeList,
			Elem:     xySchema,
			MaxItems: 1,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindObject, p.kind)
		assert.Equal(t, "config.prop", p.properties[0].parentPath.String())
	})

	t.Run("SetNestedBlock", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type: shim.TypeSet,
			Elem: xySchema,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindSet, p.kind)
		assert.Equal(t, kindObject, p.element.kind)
		assert.Equal(t, "config.prop.$", p.element.properties[0].parentPath.String())
	})

	t.Run("SetNestedBlockMaxItem1", func(t *testing.T) {
		objType := (&shimschema.Schema{
			Type:     shim.TypeSet,
			Elem:     xySchema,
			MaxItems: 1,
		}).Shim()
		p, err := g.makePropertyType(path, "obj", objType, nil, false, entityDocs{})
		require.NoError(t, err)
		assert.Equal(t, kindObject, p.kind)
		assert.Equal(t, "config.prop", p.properties[0].parentPath.String())
	})
}

func Test_ProviderWithOmittedTypes(t *testing.T) {
	t.Parallel()

	gen := func(t *testing.T, f func(*tfbridge.ResourceInfo)) pschema.PackageSpec {
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

	t.Run("no-omit", func(t *testing.T) {
		spec := gen(t, nil)
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Resources["test:index:Bar"].InputProperties, 1)
		assert.Len(t, spec.Types, 2)
	})

	t.Run("omit-top-level-prop", func(t *testing.T) {
		spec := gen(t, func(info *tfbridge.ResourceInfo) {
			info.Fields = map[string]*tfbridge.SchemaInfo{
				"obj": {Omit: true},
			}
		})
		assert.Len(t, spec.Resources, 1)
		assert.Len(t, spec.Resources["test:index:Bar"].InputProperties, 0)
		assert.Len(t, spec.Types, 0)
	})

	t.Run("omit-nested-prop", func(t *testing.T) {
		spec := gen(t, func(info *tfbridge.ResourceInfo) {
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

func TestBridgeOmitsWriteOnlyFields(t *testing.T) {
	t.Parallel()
	p := (&shimschema.Provider{
		ResourcesMap: shimschema.ResourceMap{
			"test_res_with_wo": (&shimschema.Resource{
				Schema: shimschema.SchemaMap{
					"password_wo": (&shimschema.Schema{
						Type:      shim.TypeString,
						WriteOnly: true,
						Optional:  true,
					}).Shim(),
				},
			}).Shim(),
			"test_res_no_wo": (&shimschema.Resource{
				Schema: shimschema.SchemaMap{
					"password_regular": (&shimschema.Schema{
						Type:     shim.TypeString,
						Optional: true,
					}).Shim(),
				},
			}).Shim(),
		},
	}).Shim()
	resWO := &tfbridge.ResourceInfo{
		Tok: "test:index:WriteOnly",
	}
	resNoWO := &tfbridge.ResourceInfo{
		Tok: "test:index:NoWriteOnly",
	}
	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	})
	schemaResult, err := GenerateSchemaWithOptions(GenerateSchemaOptions{
		DiagnosticsSink: nilSink,
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "test",
			P:    p,
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res_with_wo": resWO,
				"test_res_no_wo":   resNoWO,
			},
		},
	})
	require.NoError(t, err)

	spec := schemaResult.PackageSpec
	assert.Len(t, spec.Resources, 2)
	assert.Len(t, spec.Resources["test:index:WriteOnly"].InputProperties, 0)
	assert.Len(t, spec.Resources["test:index:NoWriteOnly"].InputProperties, 1)
}

func TestOmitWriteOnlyFieldsErrorWhenNotOptional(t *testing.T) {
	t.Parallel()
	p := (&shimschema.Provider{
		ResourcesMap: shimschema.ResourceMap{
			"test_res_wo": (&shimschema.Resource{
				Schema: shimschema.SchemaMap{
					"password_wo": (&shimschema.Schema{
						Type:      shim.TypeString,
						WriteOnly: true,
						Required:  true,
					}).Shim(),
				},
			}).Shim(),
		},
	}).Shim()
	resWO := &tfbridge.ResourceInfo{
		Tok: "test:index:WriteOnly",
	}
	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	})
	_, err := GenerateSchemaWithOptions(GenerateSchemaOptions{
		DiagnosticsSink: nilSink,
		ProviderInfo: tfbridge.ProviderInfo{
			Name: "test",
			P:    p,
			Resources: map[string]*tfbridge.ResourceInfo{
				"test_res_wo": resWO,
			},
		},
	})
	require.Error(t, err)
	//nolint:lll
	require.ErrorContains(t, err, "required property \"password_wo[pulumi:\\\"passwordWo\\\"]\" (@ resource[key=\"test_res_wo\",token=\"test:index:WriteOnly\"].outputs.password_wo[pulumi:\"passwordWo\"]) may not be omitted from binding generation\n\n")
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
	t.Parallel()
	assert.Equal(t, "http", withoutPackageName("http", "http"))
	assert.Equal(t, "s3_bucket", withoutPackageName("aws", "aws_s3_bucket"))
}

func TestGetNestedDescriptionFromParsedDocs(t *testing.T) {
	t.Parallel()
	testEntityDoc := entityDocs{
		Description: "This is a test resource description",
		Arguments: map[docsPath]*argumentDocs{
			"configuration":             {description: "Configuration block for broker configuration."},
			"configuration.revision":    {description: "Revision of the Configuration."},
			"configuration.revision.id": {description: "ID of the Revision of the Configuration."},
		},
		Attributes: map[string]string{
			"instances":            "List of information about allocated brokers (both active & standby).",
			"instances.ip_address": "IP Address of the broker.",
		},
		Import: "Please do not import this resource. It will not work.",
	}

	type testCase struct {
		name     string
		path     docsPath
		expected string
	}

	testCases := []testCase{
		{
			name:     "Argument Path Populates",
			path:     docsPath("configuration"),
			expected: "Configuration block for broker configuration.",
		},
		{
			name:     "Nested Argument Path Populates",
			path:     docsPath("configuration.revision"),
			expected: "Revision of the Configuration.",
		},
		{
			name:     "Double Nested Argument Path Populates",
			path:     docsPath("configuration.revision.id"),
			expected: "ID of the Revision of the Configuration.",
		},
		{
			name:     "Attribute Description Populates",
			path:     docsPath("instances"),
			expected: "List of information about allocated brokers (both active & standby).",
		},
		{
			name:     "Nested Attribute Description Populates",
			path:     docsPath("instances.ip_address"),
			expected: "IP Address of the broker.",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual, _ := getNestedDescriptionFromParsedDocs(testEntityDoc, tc.path)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetUniqueLeafDocsDescriptions(t *testing.T) {
	t.Parallel()
	testArguments := map[docsPath]*argumentDocs{
		"configuration":             {description: "Configuration block for broker configuration."},
		"configuration.revision":    {description: "Revision of the Configuration."},
		"configuration.revision.id": {description: "ID of the Revision of the Configuration."},
		"edition.revision.id":       {description: "ID of the Edition of the Configuration."},
	}

	type testCase struct {
		name     string
		path     docsPath
		expected string
	}

	testCases := []testCase{
		{
			name:     "Nonunique leaf paths are not returned",
			path:     docsPath("id"),
			expected: "",
		},
		{
			name:     "Unique leaf paths return their Description",
			path:     docsPath("revision"),
			expected: "Revision of the Configuration.",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := getUniqueLeafDocsDescriptions(testArguments, tc.path)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
