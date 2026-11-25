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

package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/parameterize"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

// elasticstackQueryRulesetSchema builds a schema shaped like
// elasticstack_elasticsearch_query_ruleset: a list-nested "rules" attribute whose
// items have a single-nested "actions" attribute, whose "docs" is itself a
// list-nested attribute with required, underscore-prefixed "_index"/"_id"
// attributes -- mirroring the Elasticsearch Query Rules API's own field names.
func elasticstackQueryRulesetSchema() *tfprotov6.GetProviderSchemaResponse {
	docAttrs := &tfprotov6.SchemaObject{
		Nesting: tfprotov6.SchemaObjectNestingModeList,
		Attributes: []*tfprotov6.SchemaAttribute{
			{Name: "_index", Type: tftypes.String, Required: true},
			{Name: "_id", Type: tftypes.String, Required: true},
		},
	}
	actionsAttrs := &tfprotov6.SchemaObject{
		Nesting: tfprotov6.SchemaObjectNestingModeSingle,
		Attributes: []*tfprotov6.SchemaAttribute{
			{Name: "docs", Required: true, NestedType: docAttrs},
		},
	}
	rulesAttrs := &tfprotov6.SchemaObject{
		Nesting: tfprotov6.SchemaObjectNestingModeList,
		Attributes: []*tfprotov6.SchemaAttribute{
			{Name: "actions", Required: true, NestedType: actionsAttrs},
		},
	}

	return &tfprotov6.GetProviderSchemaResponse{
		ResourceSchemas: map[string]*tfprotov6.Schema{
			"elasticstack_query_ruleset": {Block: &tfprotov6.SchemaBlock{
				Attributes: []*tfprotov6.SchemaAttribute{
					{Name: "id", Type: tftypes.String, Computed: true},
					{Name: "rules", Required: true, NestedType: rulesAttrs},
				},
			}},
		},
	}
}

// TestUnderscorePrefixedRequiredFieldErrorsWithoutFix pins the regression from
// https://github.com/pulumi/pulumi-terraform-bridge/pull/3597: a schema shaped like
// elasticstack's query_ruleset (a required, underscore-prefixed property nested two
// objects deep) fails schema generation when no SchemaInfo.Name mapping is present.
// This is the behavior fixUnderscorePrefixedFields exists to work around for
// dynamically bridged providers, which have no hand-authored Go code where a
// maintainer could add that mapping.
func TestUnderscorePrefixedRequiredFieldErrorsWithoutFix(t *testing.T) {
	t.Parallel()

	p, err := providerInfo(context.Background(), schemaOnlyProvider{
		name:    "elasticstack",
		version: "1.0.0",
		schema:  elasticstackQueryRulesetSchema(),
	}, parameterize.Value{Local: &parameterize.LocalValue{Path: "/dev/null"}})
	require.NoError(t, err)

	// Undo the fix to confirm the precondition it works around still holds against
	// the current tfgen behavior: a required underscore-prefixed property with no
	// explicit Name mapping aborts schema generation.
	for _, res := range p.Resources {
		res.Fields = nil
	}

	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	_, err = tfgen.GenerateSchema(p, nilSink)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "_id")
}

// TestFixUnderscorePrefixedFields proves the fix end-to-end: providerInfo (which now
// calls fixUnderscorePrefixedFields) produces a ProviderInfo that schema-generates
// successfully for a schema shaped like elasticstack's query_ruleset, with the
// underscore-prefixed properties renamed rather than dropped or erroring.
func TestFixUnderscorePrefixedFields(t *testing.T) {
	t.Parallel()

	p, err := providerInfo(context.Background(), schemaOnlyProvider{
		name:    "elasticstack",
		version: "1.0.0",
		schema:  elasticstackQueryRulesetSchema(),
	}, parameterize.Value{Local: &parameterize.LocalValue{Path: "/dev/null"}})
	require.NoError(t, err)

	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	spec, err := tfgen.GenerateSchema(p, nilSink)
	require.NoError(t, err)

	tok := p.Resources["elasticstack_query_ruleset"].Tok
	res, ok := spec.Resources[string(tok)]
	require.True(t, ok, "expected resource %q in generated schema", tok)

	// lookupType resolves a TypeSpec's $ref (directly, or via Items for an array)
	// against spec.Types, stripping the "#/types/" prefix used in refs.
	lookupType := func(ps pschema.PropertySpec) pschema.ComplexTypeSpec {
		ts := ps.TypeSpec
		t.Helper()
		ref := ts.Ref
		if ref == "" && ts.Items != nil {
			ref = ts.Items.Ref
		}
		require.NotEmpty(t, ref, "expected a type reference")
		token := strings.TrimPrefix(ref, "#/types/")
		typ, ok := spec.Types[token]
		require.True(t, ok, "expected type %q in generated schema", token)
		return typ
	}

	rules, ok := res.InputProperties["rules"]
	require.True(t, ok, "expected \"rules\" property")
	rulesItem := lookupType(rules)

	actions, ok := rulesItem.Properties["actions"]
	require.True(t, ok, "expected \"actions\" property on rules items")
	actionsType := lookupType(actions)

	docs, ok := actionsType.Properties["docs"]
	require.True(t, ok, "expected \"docs\" property on actions")
	docsItem := lookupType(docs)

	_, hasIndex := docsItem.Properties["index"]
	_, hasID := docsItem.Properties["id"]
	assert.True(t, hasIndex, "expected \"_index\" to be renamed to \"index\"")
	assert.True(t, hasID, "expected \"_id\" to be renamed to \"id\"")
}

// TestMergeUnderscoreFields unit-tests the merge/collision logic in
// mergeUnderscoreFields directly against hand-built shim schemas.
func TestMergeUnderscoreFields(t *testing.T) {
	t.Parallel()

	t.Run("adds a Name for a top-level underscore-prefixed property", func(t *testing.T) {
		t.Parallel()
		schemaMap := shimschema.SchemaMap{
			"_id":    (&shimschema.Schema{Type: shim.TypeString, Required: true}).Shim(),
			"normal": (&shimschema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		}

		result := mergeUnderscoreFields(nil, schemaMap)
		require.NotNil(t, result)
		require.NotNil(t, result["_id"])
		assert.Equal(t, "id", result["_id"].Name)
		assert.Nil(t, result["normal"])
	})

	t.Run("never overrides an existing mapping", func(t *testing.T) {
		t.Parallel()
		schemaMap := shimschema.SchemaMap{
			"_id": (&shimschema.Schema{Type: shim.TypeString, Required: true}).Shim(),
		}
		existing := map[string]*tfbridge.SchemaInfo{
			"_id": {Name: "customId"},
		}

		result := mergeUnderscoreFields(existing, schemaMap)
		require.NotNil(t, result["_id"])
		assert.Equal(t, "customId", result["_id"].Name)
	})

	t.Run("spells out underscore on collision with a sibling", func(t *testing.T) {
		t.Parallel()
		schemaMap := shimschema.SchemaMap{
			"_index": (&shimschema.Schema{Type: shim.TypeString, Required: true}).Shim(),
			"index":  (&shimschema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		}

		result := mergeUnderscoreFields(nil, schemaMap)
		require.NotNil(t, result["_index"])
		assert.Equal(t, "underscoreIndex", result["_index"].Name)
	})

	t.Run("recurses into single-nested object blocks", func(t *testing.T) {
		t.Parallel()
		nested := shimschema.SchemaMap{
			"_id": (&shimschema.Schema{Type: shim.TypeString, Required: true}).Shim(),
		}
		schemaMap := shimschema.SchemaMap{
			"actions": (&shimschema.Schema{
				Type: shim.TypeMap,
				Elem: (&shimschema.Resource{Schema: nested}).Shim(),
			}).Shim(),
		}

		result := mergeUnderscoreFields(nil, schemaMap)
		require.NotNil(t, result["actions"])
		require.NotNil(t, result["actions"].Elem)
		require.NotNil(t, result["actions"].Elem.Fields["_id"])
		assert.Equal(t, "id", result["actions"].Elem.Fields["_id"].Name)
	})

	t.Run("recurses into list-nested object blocks", func(t *testing.T) {
		t.Parallel()
		nested := shimschema.SchemaMap{
			"_id": (&shimschema.Schema{Type: shim.TypeString, Required: true}).Shim(),
		}
		schemaMap := shimschema.SchemaMap{
			"docs": (&shimschema.Schema{
				Type: shim.TypeList,
				Elem: (&shimschema.Resource{Schema: nested}).Shim(),
			}).Shim(),
		}

		result := mergeUnderscoreFields(nil, schemaMap)
		require.NotNil(t, result["docs"])
		require.NotNil(t, result["docs"].Elem)
		require.NotNil(t, result["docs"].Elem.Fields["_id"])
		assert.Equal(t, "id", result["docs"].Elem.Fields["_id"].Name)
	})

	t.Run("preserves existing nested fields alongside the auto-added ones", func(t *testing.T) {
		t.Parallel()
		nested := shimschema.SchemaMap{
			"_id":   (&shimschema.Schema{Type: shim.TypeString, Required: true}).Shim(),
			"other": (&shimschema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		}
		schemaMap := shimschema.SchemaMap{
			"docs": (&shimschema.Schema{
				Type: shim.TypeList,
				Elem: (&shimschema.Resource{Schema: nested}).Shim(),
			}).Shim(),
		}
		existing := map[string]*tfbridge.SchemaInfo{
			"docs": {
				Elem: &tfbridge.SchemaInfo{
					Fields: map[string]*tfbridge.SchemaInfo{
						"other": {Name: "renamedOther"},
					},
				},
			},
		}

		result := mergeUnderscoreFields(existing, schemaMap)
		require.NotNil(t, result["docs"].Elem.Fields["_id"])
		assert.Equal(t, "id", result["docs"].Elem.Fields["_id"].Name)
		require.NotNil(t, result["docs"].Elem.Fields["other"])
		assert.Equal(t, "renamedOther", result["docs"].Elem.Fields["other"].Name)
	})

	t.Run("returns nil for an empty schema map", func(t *testing.T) {
		t.Parallel()
		result := mergeUnderscoreFields(nil, shimschema.SchemaMap{})
		assert.Nil(t, result)
	})
}
