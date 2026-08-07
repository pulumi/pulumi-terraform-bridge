// Copyright 2016-2026, Pulumi Corporation.
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

package tfbridgetests

import (
	"context"
	"testing"

	tfdiag "github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// Terraform providers occasionally declare fields whose names begin with an underscore.
// These are effectively private/internal to the provider. The bridge should:
//   - Omit them from the generated Pulumi schema entirely.
//   - Roundtrip them through TF state so that values set during Create are visible to
//     subsequent Update calls on the TF side.
func TestUnderscorePrefixedField(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const privateVal = "secret-value"
	const mangledVal = "mangled-value"
	var seenPrivateOnUpdate, seenMangledOnUpdate string

	tfProvider := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"example_resource": {
				Schema: map[string]*schema.Schema{
					"name":     {Type: schema.TypeString, Optional: true},
					"_private": {Type: schema.TypeString, Computed: true},
					// A double-underscore field would collide with Python's name-mangling
					// rules if surfaced to the Pulumi schema; it must also be dropped.
					"__mangled": {Type: schema.TypeString, Computed: true},
				},
				CreateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) tfdiag.Diagnostics {
					rd.SetId("id0")
					_ = rd.Set("_private", privateVal)
					_ = rd.Set("__mangled", mangledVal)
					return nil
				},
				UpdateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) tfdiag.Diagnostics {
					if v, ok := rd.GetOk("_private"); ok {
						seenPrivateOnUpdate, _ = v.(string)
					}
					if v, ok := rd.GetOk("__mangled"); ok {
						seenMangledOnUpdate, _ = v.(string)
					}
					return nil
				},
				ReadContext: func(_ context.Context, _ *schema.ResourceData, _ interface{}) tfdiag.Diagnostics {
					return nil
				},
			},
		},
	}

	provInfo := tfbridge.ProviderInfo{
		P:              shimv2.NewProvider(tfProvider),
		Name:           "testprov",
		ResourcePrefix: "example",
		Resources: map[string]*info.Resource{
			"example_resource": {Tok: "testprov:index:ExampleResource"},
		},
	}

	// 1. Schema check: neither the raw `_private` name nor its camelCased form
	//    should appear on the Pulumi resource.
	t.Run("not in pulumi schema", func(t *testing.T) {
		packageSpec, err := tfgen.GenerateSchema(provInfo, nilSink())
		require.NoError(t, err)
		res, ok := packageSpec.Resources["testprov:index:ExampleResource"]
		require.True(t, ok, "expected resource in generated schema")
		for _, forbidden := range []string{
			"_private", "private", "Private",
			"__mangled", "mangled", "Mangled",
		} {
			_, hasInput := res.InputProperties[forbidden]
			assert.Falsef(t, hasInput, "InputProperties must not contain %q", forbidden)
			_, hasOutput := res.Properties[forbidden]
			assert.Falsef(t, hasOutput, "Properties must not contain %q", forbidden)
		}
	})

	// 2. Runtime roundtrip: Create sets `_private` on TF state; Update must see it
	//    in the prior state even though it is never exposed as a Pulumi property.
	t.Run("roundtrips through state to update", func(t *testing.T) {
		p := newTestProvider(ctx, provInfo, newTestProviderOptions{})

		urn := "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres"

		createIns, err := plugin.MarshalProperties(resource.PropertyMap{
			"name": resource.NewStringProperty("foo"),
		}, plugin.MarshalOptions{})
		require.NoError(t, err)

		createResp, err := p.Create(ctx, &pulumirpc.CreateRequest{
			Urn:        urn,
			Properties: createIns,
		})
		require.NoError(t, err)

		createOuts, err := plugin.UnmarshalProperties(createResp.GetProperties(), plugin.MarshalOptions{
			KeepUnknowns: true, SkipNulls: true,
		})
		require.NoError(t, err)

		// The camelCased/PascalCased forms must never appear (they belong to the schema).
		for _, forbidden := range []resource.PropertyKey{"private", "Private"} {
			_, present := createOuts[forbidden]
			assert.Falsef(t, present, "Create outputs must not contain %q", forbidden)
		}
		// The raw underscore-prefixed keys are expected to pass through so they can
		// roundtrip back to the TF provider on Update.
		assert.Equal(t, resource.NewStringProperty(privateVal), createOuts["_private"],
			"Create outputs should carry _private through unchanged for roundtripping")
		assert.Equal(t, resource.NewStringProperty(mangledVal), createOuts["__mangled"],
			"Create outputs should carry __mangled through unchanged for roundtripping")

		updateNews, err := plugin.MarshalProperties(resource.PropertyMap{
			"name": resource.NewStringProperty("bar"),
		}, plugin.MarshalOptions{})
		require.NoError(t, err)

		_, err = p.Update(ctx, &pulumirpc.UpdateRequest{
			Id:   "id0",
			Urn:  urn,
			Olds: createResp.GetProperties(),
			News: updateNews,
		})
		require.NoError(t, err)

		assert.Equal(t, privateVal, seenPrivateOnUpdate,
			"TF UpdateContext should observe the _private value set during Create")
		assert.Equal(t, mangledVal, seenMangledOnUpdate,
			"TF UpdateContext should observe the __mangled value set during Create")
	})

	// 3. An explicit SchemaInfo.Name override for an underscore-prefixed key should win:
	//    the drop rule must defer to user overrides.
	t.Run("explicit name override wins", func(t *testing.T) {
		overrideInfo := provInfo
		overrideInfo.Resources = map[string]*info.Resource{
			"example_resource": {
				Tok: "testprov:index:ExampleResource",
				Fields: map[string]*info.Schema{
					"_private": {Name: "renamedPrivate"},
				},
			},
		}
		packageSpec, err := tfgen.GenerateSchema(overrideInfo, nilSink())
		require.NoError(t, err)
		res, ok := packageSpec.Resources["testprov:index:ExampleResource"]
		require.True(t, ok)
		_, has := res.Properties["renamedPrivate"]
		assert.True(t, has, "explicit Name override should surface the property in the schema")
	})

	// 4. A required underscore-prefixed field cannot be silently dropped; schema
	//    generation should error, mirroring the SchemaInfo.Omit behavior.
	t.Run("required underscore field errors", func(t *testing.T) {
		requiredProv := &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"example_resource": {
					Schema: map[string]*schema.Schema{
						"_private": {Type: schema.TypeString, Required: true},
					},
				},
			},
		}
		badInfo := tfbridge.ProviderInfo{
			P:              shimv2.NewProvider(requiredProv),
			Name:           "testprov",
			ResourcePrefix: "example",
			Resources: map[string]*info.Resource{
				"example_resource": {Tok: "testprov:index:ExampleResource"},
			},
		}
		_, err := tfgen.GenerateSchema(badInfo, nilSink())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "may not be omitted from binding generation")
	})
}
