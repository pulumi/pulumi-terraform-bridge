package tfgen

import (
	"io"
	"testing"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/testprovider"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type mockDeclarer struct {
	name string
}

func (m mockDeclarer) Name() string {
	return m.name
}

func generateTestResource(providerName, resName string) *paths.ResourcePath {
	name, moduleName := resourceName(providerName, resName, nil, false)
	mod := tokens.NewModuleToken(
		tokens.NewPackageToken(tokens.PackageName(providerName)),
		moduleName,
	)
	resourceToken := tokens.NewTypeToken(mod, name)
	return paths.NewResourcePath(resName, resourceToken, false)
}

func testGenerator(
	t *testing.T,
	provider info.Provider,
	providerName, tfResourceName string,
) (*Generator, *paths.ResourcePath, shim.Resource, *info.Resource) {
	g, err := NewGenerator(GeneratorOptions{
		Package:      provider.Name,
		Version:      provider.Version,
		Language:     Schema,
		ProviderInfo: provider,
		Root:         afero.NewMemMapFs(),
		Sink:         diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
	})
	assert.NoError(t, err)
	resourcePath := generateTestResource(providerName, tfResourceName)
	resources := g.provider().ResourcesMap()
	resourceSchema := resources.Get(tfResourceName)
	resourceInfo := provider.Resources[tfResourceName]
	return g, resourcePath, resourceSchema, resourceInfo
}

type typePathSet struct {
	set paths.TypePathSet
}

func (t *typePathSet) Add(typePath paths.TypePath) *typePathSet {
	t.set.Add(typePath)
	return t
}

func testCreateTypePathSet(resourcePath paths.ResourcePath, propertyName string) *typePathSet {
	set := paths.NewTypePathSet()
	set.Add(paths.NewProperyPath(resourcePath.Inputs(), paths.PropertyName{
		Key:  propertyName,
		Name: tokens.Name(propertyName),
	}))
	return &typePathSet{
		set: set,
	}
}

func Test_schemaNestedTypes_getNameForTypeNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		in           nestedTypeGraph
		propertyName string
		nameToType   map[string]*schemaNestedType
		expected     string
		declarer     declarer
		typ          *propertyType
	}{
		{
			name: "name < 120 picks longest name",
			in: newNestedTypeGraph("Template").
				createBranch("definition").
				createBranch("analysisDefaults").
				createBranch("defaultNewSheetConfiguration"),
			propertyName: "defaultNewSheetConfiguration",
			nameToType:   map[string]*schemaNestedType{},
			expected:     "TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfiguration",
			declarer:     mockDeclarer{name: "template"},
			typ:          &propertyType{},
		},
		{
			name: "name > 120 picks shortest name",
			in: newNestedTypeGraph("Template").
				createBranch("definition").
				createBranch("analysisDefaultsDefaultsDefaultsDefaultsDefaultsDefaultsDefaultsDefaultsDefaults").
				createBranch("defaultNewSheetConfiguration"),
			propertyName: "defaultNewSheetConfiguration",
			nameToType:   map[string]*schemaNestedType{},
			expected:     "TemplateDefinitionDefaultNewSheetConfiguration",
			declarer:     mockDeclarer{name: "template"},
			typ:          &propertyType{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			nt := &schemaNestedTypes{
				nameToType: tc.nameToType,
			}

			actual := nt.getNameForTypeNode(tc.in, tc.propertyName, tc.declarer, tc.typ)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_schemaNestedTypes_getShortestName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		in           nestedTypeGraph
		propertyName string
		nameToType   map[string]*schemaNestedType
		expected     string
		declarer     declarer
		typ          *propertyType
	}{
		{
			name:         "top level property",
			in:           newNestedTypeGraph("Template").createBranch("definition"),
			propertyName: "definition",
			nameToType:   map[string]*schemaNestedType{},
			expected:     "TemplateDefinition",
			declarer:     mockDeclarer{name: "template"},
			typ:          &propertyType{},
		},
		{
			// this will eventually fail later on, which is why we don't fail in `getShortestName`
			name:         "returns only available name",
			in:           newNestedTypeGraph("Template").createBranch("definition"),
			propertyName: "definition",
			nameToType: map[string]*schemaNestedType{
				"TemplateDefinition": {
					declarer: mockDeclarer{name: "other"},
				},
			},
			expected: "TemplateDefinition",
			declarer: mockDeclarer{name: "template"},
			typ:      &propertyType{},
		},
		{
			name: "reuse type (same typ)",
			in: newNestedTypeGraph("Template").
				createBranch("definition").
				createBranch("analysisDefaults").
				createBranch("defaultNewSheetConfiguration"),
			propertyName: "defaultNewSheetConfiguration",
			nameToType: map[string]*schemaNestedType{
				// already taken
				"TemplateDefinitionDefaultNewSheetConfiguration": {
					declarer: mockDeclarer{name: "template"},
					// typ is different
					typ: &propertyType{name: "other"},
				},
			},
			expected: "TemplateDefinitionDefaultNewSheetConfiguration",
			declarer: mockDeclarer{name: "template"},
			typ:      &propertyType{},
		},
		{
			name: "reuse type (same declarer)",
			in: newNestedTypeGraph("Template").
				createBranch("definition").
				createBranch("analysisDefaults").
				createBranch("defaultNewSheetConfiguration"),
			propertyName: "defaultNewSheetConfiguration",
			nameToType: map[string]*schemaNestedType{
				// already taken
				"TemplateDefinitionDefaultNewSheetConfiguration": {
					declarer: mockDeclarer{name: "other"},
					typ:      &propertyType{},
				},
			},
			expected: "TemplateDefinitionDefaultNewSheetConfiguration",
			declarer: mockDeclarer{name: "template"},
			typ:      &propertyType{},
		},
		{
			name: "shortest name not available",
			in: newNestedTypeGraph("Template").
				createBranch("definition").
				createBranch("analysisDefaults").
				createBranch("defaultNewSheetConfiguration"),
			propertyName: "defaultNewSheetConfiguration",
			nameToType: map[string]*schemaNestedType{
				// already taken
				"TemplateDefinitionDefaultNewSheetConfiguration": {
					declarer: mockDeclarer{name: "other"},
					// typ is different
					typ: &propertyType{name: "other"},
				},
			},
			expected: "TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfiguration",
			declarer: mockDeclarer{name: "template"},
			typ:      &propertyType{},
		},
		{
			name: "reuse type",
			in: newNestedTypeGraph("Template").
				createBranch("definition").
				createBranch("analysisDefaults").
				createBranch("defaultNewSheetConfiguration"),
			propertyName: "defaultNewSheetConfiguration",
			nameToType: map[string]*schemaNestedType{
				// already taken
				"TemplateDefinitionDefaultNewSheetConfiguration": {
					// declarer is the same
					declarer: mockDeclarer{name: "template"},
					// typ is same
					typ: &propertyType{},
				},
			},
			expected: "TemplateDefinitionDefaultNewSheetConfiguration",
			declarer: mockDeclarer{name: "template"},
			typ:      &propertyType{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			nt := &schemaNestedTypes{
				nameToType: tc.nameToType,
			}

			actual := nt.getShortestName(tc.propertyName, tc.in, tc.declarer, tc.typ)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_schemaNestedTypes_declareType(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderLargeTokens()
	tfResourceName := "aws_quicksight_template"
	g, resourcePath, resourceSchema, resourceInfo := testGenerator(t, provider, "aws", tfResourceName)
	tests := []struct {
		name               string
		in                 nestedTypeGraph
		propertyName       string
		startingNameToType map[string]*schemaNestedType
		expectedNameToType map[string]*schemaNestedType
		expected           nestedTypeGraph
	}{
		{
			name:               "top level property",
			in:                 newNestedTypeGraph("Template"),
			propertyName:       "definition",
			startingNameToType: map[string]*schemaNestedType{},
			expectedNameToType: map[string]*schemaNestedType{
				"TemplateDefinition": {
					typ: &propertyType{
						name: "TemplateDefinition",
					},
					declarer:  mockDeclarer{name: "template"},
					typePaths: testCreateTypePathSet(*resourcePath, "definition").set,
				},
			},
			expected: nestedTypeGraph{
				paths:       []string{"TemplateDefinition"},
				longestPath: []string{"TemplateDefinition"},
				root:        "Template",
				branches: map[string]nestedTypeNode{
					"definition": {
						paths:           []string{"TemplateDefinition"},
						longestPathName: "TemplateDefinition",
					},
				},
			},
		},
		{
			name:         "existing type",
			in:           newNestedTypeGraph("Template"),
			propertyName: "definition",
			startingNameToType: map[string]*schemaNestedType{
				"TemplateDefinition": {
					typ:       &propertyType{name: "TemplateDefinition"},
					declarer:  mockDeclarer{name: "template"},
					typePaths: testCreateTypePathSet(*resourcePath, "other").set,
				},
			},
			expectedNameToType: map[string]*schemaNestedType{
				"TemplateDefinition": {
					typ: &propertyType{
						name: "TemplateDefinition",
					},
					declarer: mockDeclarer{name: "template"},
					typePaths: testCreateTypePathSet(*resourcePath, "definition").
						Add(paths.NewProperyPath(resourcePath.Inputs(), paths.PropertyName{Name: "other", Key: "other"})).set,
				},
			},
			expected: nestedTypeGraph{
				paths:       []string{"TemplateDefinition"},
				longestPath: []string{"TemplateDefinition"},
				root:        "Template",
				branches: map[string]nestedTypeNode{
					"definition": {
						paths:           []string{"TemplateDefinition"},
						longestPathName: "TemplateDefinition",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			variable, err := g.propertyVariable(
				resourcePath.Inputs(),
				tc.propertyName,
				resourceSchema.Schema(),
				resourceInfo.Fields,
				"",
				"",
				false,
				entityDocs{},
			)
			assert.NoError(t, err)
			nt := &schemaNestedTypes{
				nameToType: tc.startingNameToType,
			}

			typePath := paths.NewProperyPath(variable.parentPath, variable.propertyName)
			actual := nt.declareType(
				typePath,
				mockDeclarer{name: "template"},
				tc.in,
				tc.propertyName,
				variable.typ,
				true,
			)
			assert.Equal(t, tc.expected, actual)

			for key, value := range tc.expectedNameToType {
				val, ok := nt.nameToType[key]
				if !ok {
					t.Fatalf("Expected name %s not found in nameToType", key)
				}
				assert.Equal(t, value.declarer, val.declarer)
				assert.Equal(t, value.typ.name, val.typ.name)
				assert.Equal(t, value.typePaths, val.typePaths)
			}
		})
	}
}

func Test_nestedNodeGraph_createBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in       nestedTypeGraph
		branch   string
		expected nestedTypeGraph
	}{
		{
			name:     "root",
			in:       newNestedTypeGraph("Template"),
			branch:   "Template",
			expected: newNestedTypeGraph("Template"),
		},
		{
			name:   "top level",
			in:     newNestedTypeGraph("Template"),
			branch: "definition",
			expected: nestedTypeGraph{
				root:        "Template",
				paths:       []string{"TemplateDefinition"},
				longestPath: []string{"TemplateDefinition"},
				branches: map[string]nestedTypeNode{
					"definition": {
						paths:           []string{"TemplateDefinition"},
						longestPathName: "TemplateDefinition",
					},
				},
			},
		},
		{
			name: "nested type",
			in: newNestedTypeGraph("Template").
				createBranch("definition").
				createBranch("analysisDefaults").
				createBranch("defaultNewSheetConfiguration"),
			branch: "sectionBased",
			expected: nestedTypeGraph{
				root: "Template",
				paths: []string{
					"TemplateDefinition",
					"TemplateDefinitionAnalysisDefaults",
					"TemplateDefinitionDefaultNewSheetConfiguration",
					"TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfiguration",
					"TemplateDefinitionSectionBased",
					"TemplateDefinitionAnalysisDefaultsSectionBased",
					"TemplateDefinitionDefaultNewSheetConfigurationSectionBased",
					"TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfigurationSectionBased",
				},
				longestPath: []string{
					"TemplateDefinition",
					"TemplateDefinitionAnalysisDefaults",
					"TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfiguration",
					"TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfigurationSectionBased",
				},
				branches: map[string]nestedTypeNode{
					"definition": {
						paths:           []string{"TemplateDefinition"},
						longestPathName: "TemplateDefinition",
					},
					"analysisDefaults": {
						paths:           []string{"TemplateDefinitionAnalysisDefaults"},
						longestPathName: "TemplateDefinitionAnalysisDefaults",
					},
					"defaultNewSheetConfiguration": {
						paths: []string{
							"TemplateDefinitionDefaultNewSheetConfiguration",
							"TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfiguration",
						},
						longestPathName: "TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfiguration",
					},
					"sectionBased": {
						paths: []string{
							"TemplateDefinitionSectionBased",
							"TemplateDefinitionAnalysisDefaultsSectionBased",
							"TemplateDefinitionDefaultNewSheetConfigurationSectionBased",
							"TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfigurationSectionBased",
						},
						longestPathName: "TemplateDefinitionAnalysisDefaultsDefaultNewSheetConfigurationSectionBased",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out := tc.in.createBranch(tc.branch)
			assert.Equal(t, tc.expected, out)
		})
	}
}
