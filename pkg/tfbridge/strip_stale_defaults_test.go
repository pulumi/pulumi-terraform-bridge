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

package tfbridge

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestStripStaleDefaults(t *testing.T) {
	t.Parallel()

	// Helper to build a SchemaMap from a map of schema definitions.
	makeSchemaMap := func(schemas map[string]shim.Schema) shim.SchemaMap {
		return schema.SchemaMap(schemas)
	}

	t.Run("no __defaults key returns unchanged", func(t *testing.T) {
		m := resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"foo": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		assert.Equal(t, m, result)
	})

	t.Run("empty __defaults array returns unchanged", func(t *testing.T) {
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{}),
			"foo":                 resource.NewStringProperty("bar"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"foo": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		assert.Equal(t, m, result)
	})

	t.Run("stale default stripped when TF schema no longer has a Default", func(t *testing.T) {
		// Field listed in __defaults whose schema no longer has a Default — the
		// recorded value is genuinely stale and would otherwise reach
		// PlanResourceChange. This is the original motivating case (e.g. AWS
		// auth_token_update_strategy in v6→v7).
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("staleField"),
			}),
			"staleField": resource.NewStringProperty("ROTATE"),
			"otherField": resource.NewStringProperty("keep"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"stale_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
			"other_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasStale := result["staleField"]
		assert.False(t, hasStale, "stale default should be stripped when schema has no current Default")
		assert.Equal(t, resource.NewStringProperty("keep"), result["otherField"])
		_, hasDefaults := result[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "__defaults should be removed when empty")
	})

	t.Run("field with bridge Default kept", func(t *testing.T) {
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("name"),
			}),
			"name": resource.NewStringProperty("auto-generated-name"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"name": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"name": {Default: &info.Default{
				Value: "auto-default",
			}},
		}
		result := stripStaleDefaults(m, tfs, ps)
		assert.Equal(t, m, result)
	})

	t.Run("field with current TF Default is preserved", func(t *testing.T) {
		// When the TF schema still has a Default for the field, the stored value is
		// preserved (not stripped). Two reasons:
		//   1. PR #3420 (TestUpdatePreservesLegacyFalsyTFDefaults) requires that
		//      stored falsy values for fields whose schema has a matching Default
		//      reach RawConfig as cty.False/0/"" rather than null — providers read
		//      RawConfig presence as meaningful.
		//   2. If the schema's Default value has changed (v1 → v2), stripping in Diff
		//      alone would produce a Preview transition that Update silently fails to
		//      apply (TF treats unstripped Update news as authoritative). Preserving
		//      the stored value keeps the (silent) pre-PR baseline behavior intact;
		//      see issue #3434 for the architectural fix.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("activeDefault"),
			}),
			"activeDefault": resource.NewStringProperty("some-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"active_default": (&schema.Schema{
				Type:     shim.TypeString,
				Optional: true,
				Default:  "current-default",
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		assert.Equal(t, m, result, "field with current TF Default must be preserved")
	})

	t.Run("field with current TF DefaultFunc is preserved (even if it would return nil)", func(t *testing.T) {
		// The strip predicate must check structural schema ownership
		// (Default/DefaultFunc), not the runtime evaluation result. A DefaultFunc
		// that returns nil at Diff time (e.g. a missing env var for an env-backed
		// default) still represents a configured Default — the field is owned by
		// the schema's default mechanism and must not be classified as stale.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("envField"),
			}),
			"envField": resource.NewStringProperty("stored-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"env_field": (&schema.Schema{
				Type:        shim.TypeString,
				Optional:    true,
				DefaultFunc: func() (any, error) { return nil, nil },
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		assert.Equal(t, m, result,
			"field whose schema declares DefaultFunc must be preserved regardless of runtime value")
	})

	t.Run("field removed from TF schema is also stripped", func(t *testing.T) {
		// makeObjectTerraformInputs does NOT drop unknown keys — it substitutes an empty
		// schema and forwards the value to the provider. So a field listed in __defaults
		// but no longer in the TF schema must be stripped here, otherwise its stale value
		// would reach PlanResourceChange as an unknown attribute.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("removedField"),
			}),
			"removedField": resource.NewStringProperty("old-value"),
			"keptField":    resource.NewStringProperty("present"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"kept_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasRemoved := result["removedField"]
		assert.False(t, hasRemoved, "field absent from current TF schema should be stripped")
		assert.Equal(t, resource.NewStringProperty("present"), result["keptField"])
	})

	t.Run("field marked Removed in TF schema is stripped even with a Default", func(t *testing.T) {
		// A field can still appear in the TF schema with `Removed != ""` to signal
		// "no longer accepted." applyDefaults already skips Removed fields when applying
		// defaults; the strip mirrors that so a stale stored value cannot leak through
		// to PlanResourceChange even if the schema author left a Default attached.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("retiredField"),
			}),
			"retiredField": resource.NewStringProperty("old-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"retired_field": (&schema.Schema{
				Type:     shim.TypeString,
				Optional: true,
				Default:  "stale-default",
				Removed:  "use new_field instead",
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasRetired := result["retiredField"]
		assert.False(t, hasRetired,
			"a Removed field must be stripped even when the schema still has a Default attached")
	})

	t.Run("field marked Removed in bridge SchemaInfo is stripped", func(t *testing.T) {
		// A bridge SchemaInfo.Removed (overlay-level removal) must also strip the
		// stored value, mirroring applyDefaults' overlay-branch skip at schema.go:772.
		// This guards against bridge-overlay deprecation paths leaking stale values.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("renamedField"),
			}),
			"renamedField": resource.NewStringProperty("old-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"renamed_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"renamed_field": {Removed: true},
		}
		result := stripStaleDefaults(m, tfs, ps)
		_, hasField := result["renamedField"]
		assert.False(t, hasField, "a bridge-Removed field must be stripped")
	})

	t.Run("bridge psi.Removed shadows current TF Default (parity with applyDefaults)", func(t *testing.T) {
		// applyDefaults skips both branches when psi.Removed is set (overlay branch
		// at schema.go via defaultExcluded; TF branch likewise). The strip must
		// mirror that — even if the TF schema still has a current Default, a
		// bridge-Removed field must not have its stale value forwarded.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("renamedField"),
			}),
			"renamedField": resource.NewStringProperty("old-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"renamed_field": (&schema.Schema{
				Type:     shim.TypeString,
				Optional: true,
				Default:  "current-tf-default",
			}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"renamed_field": {Removed: true},
		}
		result := stripStaleDefaults(m, tfs, ps)
		_, hasField := result["renamedField"]
		assert.False(t, hasField,
			"bridge psi.Removed must take precedence over current TF Default")
	})

	t.Run("TF Removed shadows bridge Default (parity with applyDefaults)", func(t *testing.T) {
		// applyDefaults at schema.go:781-784 skips overlay defaulting when TF marks
		// the field Removed. classifyStaleDefault must mirror this — a stale value
		// must be stripped even when an overlay declares a Default, otherwise the
		// strip would forward the value to PlanResourceChange where SDKv2 rejects
		// the Removed attribute.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("retiredField"),
			}),
			"retiredField": resource.NewStringProperty("env-derived-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"retired_field": (&schema.Schema{
				Type:     shim.TypeString,
				Optional: true,
				Removed:  "use new_field instead",
			}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"retired_field": {Default: &info.Default{EnvVars: []string{"OLD_VAR"}}},
		}
		result := stripStaleDefaults(m, tfs, ps)
		_, hasField := result["retiredField"]
		assert.False(t, hasField,
			"TF Removed must take precedence over bridge HasDefault — stale value must be stripped")
	})

	t.Run("TF Deprecated&&!Required shadows bridge Default (parity with applyDefaults)", func(t *testing.T) {
		// applyDefaults skips overlay defaulting when TF marks the field
		// Deprecated && !Required. classifyStaleDefault must mirror this.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("legacyField"),
			}),
			"legacyField": resource.NewStringProperty("env-derived-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"legacy_field": (&schema.Schema{
				Type:       shim.TypeString,
				Optional:   true,
				Deprecated: "use new_field instead",
			}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"legacy_field": {Default: &info.Default{EnvVars: []string{"OLD_VAR"}}},
		}
		result := stripStaleDefaults(m, tfs, ps)
		_, hasField := result["legacyField"]
		assert.False(t, hasField,
			"TF Deprecated&&!Required must take precedence over bridge HasDefault — stale value must be stripped")
	})

	t.Run("deprecated optional field is stripped (mirrors applyDefaults gate)", func(t *testing.T) {
		// applyDefaults at schema.go:782 skips defaulting when a field is Deprecated
		// AND not Required. The strip mirrors this so a stale stored value isn't
		// forwarded if the provider has tightened validation around the deprecated
		// field on upgrade.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("legacyField"),
			}),
			"legacyField": resource.NewStringProperty("stale-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"legacy_field": (&schema.Schema{
				Type:       shim.TypeString,
				Optional:   true,
				Default:    "old-default",
				Deprecated: "use new_field instead",
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasField := result["legacyField"]
		assert.False(t, hasField,
			"a deprecated optional field must be stripped even with a Default attached")
	})

	t.Run("deprecated required field is preserved (cannot strip a required field)", func(t *testing.T) {
		// applyDefaults still applies defaults for deprecated-AND-required fields.
		// The strip should not strip a required field — doing so would break
		// PlanResourceChange's required-field validation. Verify the gate respects
		// `!Required`.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("requiredLegacy"),
			}),
			"requiredLegacy": resource.NewStringProperty("stored-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"required_legacy": (&schema.Schema{
				Type:       shim.TypeString,
				Required:   true,
				Default:    "old-default",
				Deprecated: "soft warning only",
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		assert.Equal(t, m, result,
			"a deprecated-but-required field must be preserved (strip would break validation)")
	})

	t.Run("non-string entry in __defaults is preserved", func(t *testing.T) {
		// __defaults should contain only string keys, but if a non-string ever appears
		// (serialization quirk, future code change), it must be preserved verbatim.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewNumberProperty(42),
				resource.NewStringProperty("staleField"),
			}),
			"staleField": resource.NewStringProperty("old-default"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"stale_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasStale := result["staleField"]
		assert.False(t, hasStale, "string entry should still be stripped")
		defaultsVal, hasDefaults := result[reservedkeys.Defaults]
		require.True(t, hasDefaults, "__defaults must remain since the non-string entry is preserved")
		require.True(t, defaultsVal.IsArray())
		defaults := defaultsVal.ArrayValue()
		assert.Len(t, defaults, 1)
		assert.True(t, defaults[0].IsNumber(), "non-string entry should be preserved")
		assert.Equal(t, float64(42), defaults[0].NumberValue())
	})

	t.Run("__defaults listing a key absent from m is handled safely", func(t *testing.T) {
		// __defaults can list a key that is not present in m (e.g. after a manual edit
		// or a partial state). The function must not panic and must update __defaults
		// to reflect the strip.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("missingField"),
			}),
			"otherField": resource.NewStringProperty("present"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"missing_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
			"other_field":   (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasDefaults := result[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "__defaults should be removed when its only entry is stripped")
		assert.Equal(t, resource.NewStringProperty("present"), result["otherField"])
	})

	t.Run("bridge Default with EnvVars (no literal value) is kept", func(t *testing.T) {
		// SchemaInfo.Default may specify EnvVars without a literal Value. HasDefault()
		// returns true in that case, so the field should be preserved (not stripped)
		// just like the literal-value case.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("region"),
			}),
			"region": resource.NewStringProperty("us-east-1"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"region": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"region": {Default: &info.Default{EnvVars: []string{"AWS_REGION"}}},
		}
		result := stripStaleDefaults(m, tfs, ps)
		assert.Equal(t, m, result, "field with EnvVars-based bridge default should be preserved")
	})

	t.Run("bridge Default with From function (no literal value) is kept", func(t *testing.T) {
		// SchemaInfo.Default with a From function (e.g. computed from other fields)
		// also reports HasDefault() == true and must be preserved.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("derivedField"),
			}),
			"derivedField": resource.NewStringProperty("from-fn"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"derived_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"derived_field": {Default: &info.Default{
				From: func(res *info.PulumiResource) (any, error) { return "computed", nil },
			}},
		}
		result := stripStaleDefaults(m, tfs, ps)
		assert.Equal(t, m, result, "field with From-based bridge default should be preserved")
	})

	t.Run("secret-wrapped scalar listed in __defaults is stripped", func(t *testing.T) {
		// A top-level field listed in __defaults whose value is a secret-wrapped scalar
		// (not an object) must still be removed from the result.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("secretField"),
			}),
			"secretField": resource.MakeSecret(resource.NewStringProperty("hidden-default")),
			"otherField":  resource.NewStringProperty("kept"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"secret_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
			"other_field":  (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasSecret := result["secretField"]
		assert.False(t, hasSecret, "secret-wrapped scalar in __defaults should be stripped")
		// __defaults bookkeeping must also reflect the strip — a buggy implementation
		// that drops the value but leaves the entry in __defaults would otherwise pass.
		_, hasDefaults := result[reservedkeys.Defaults]
		assert.False(t, hasDefaults,
			"__defaults must be removed when its only entry is the stripped key")
	})

	t.Run("mixed defaults: bridge default kept, TF-only default stripped", func(t *testing.T) {
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("staleField"),
				resource.NewStringProperty("bridgeField"),
			}),
			"staleField":  resource.NewStringProperty("old-default"),
			"bridgeField": resource.NewStringProperty("bridge-default"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"stale_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
			"bridge_field": (&schema.Schema{
				Type:     shim.TypeString,
				Optional: true,
			}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"bridge_field": {Default: &info.Default{Value: "bridge-default"}},
		}
		result := stripStaleDefaults(m, tfs, ps)
		_, hasStale := result["staleField"]
		assert.False(t, hasStale, "TF-only default should be stripped")
		assert.Equal(t, resource.NewStringProperty("bridge-default"), result["bridgeField"])
		// __defaults should only contain the bridge-defaulted field
		defaults := result[reservedkeys.Defaults].ArrayValue()
		assert.Len(t, defaults, 1)
		assert.Equal(t, "bridgeField", defaults[0].StringValue())
	})

	t.Run("all defaults stale removes __defaults key", func(t *testing.T) {
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("stale1"),
				resource.NewStringProperty("stale2"),
			}),
			"stale1": resource.NewStringProperty("val1"),
			"stale2": resource.NewStringProperty("val2"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"stale1": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
			"stale2": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasDefaults := result[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "__defaults should be removed entirely")
		_, hasStale1 := result["stale1"]
		_, hasStale2 := result["stale2"]
		assert.False(t, hasStale1)
		assert.False(t, hasStale2)
	})

	t.Run("stale default in nested object is stripped", func(t *testing.T) {
		// A stale default inside a nested block should be stripped from the nested object.
		nested := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("nestedStale"),
			}),
			"nestedStale": resource.NewStringProperty("old-default"),
			"nestedKept":  resource.NewStringProperty("user-set"),
		})
		m := resource.PropertyMap{
			"block": nested,
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"block": (&schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"nested_stale": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
						"nested_kept":  (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		blockVal := result["block"].ObjectValue()
		_, hasNestedStale := blockVal["nestedStale"]
		assert.False(t, hasNestedStale, "nested stale default should be stripped")
		assert.Equal(t, resource.NewStringProperty("user-set"), blockVal["nestedKept"])
		_, hasDefaults := blockVal[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "nested __defaults should be removed when empty")
	})

	t.Run("nested field with current TF Default is preserved", func(t *testing.T) {
		// Same predicate at the nested level: if the nested field's schema still has
		// a Default, the recorded value is preserved (not stripped). This avoids the
		// changed-default phantom diff and the PR #3420 round-trip break inside
		// nested blocks.
		nested := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("nestedActive"),
			}),
			"nestedActive": resource.NewStringProperty("still-default"),
		})
		m := resource.PropertyMap{
			"block": nested,
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"block": (&schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"nested_active": (&schema.Schema{
							Type:     shim.TypeString,
							Optional: true,
							Default:  "still-default",
						}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		assert.Equal(t, m, result, "nested field with current TF Default must be preserved")
	})

	t.Run("nested field with bridge Default is preserved", func(t *testing.T) {
		// Inside a MaxItemsOne block, a nested field with a SchemaInfo.Default must
		// not be stripped — bridge defaults flow through psi.Fields recursion. This
		// guards against a regression where the psi.HasDefault() carve-out could be
		// dropped at the nested level, which would (e.g.) break auto-naming of
		// fields nested inside blocks.
		nested := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("nestedName"),
			}),
			"nestedName": resource.NewStringProperty("bridge-generated"),
		})
		m := resource.PropertyMap{
			"block": nested,
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"block": (&schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"nested_name": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		ps := map[string]*SchemaInfo{
			"block": {Fields: map[string]*SchemaInfo{
				"nested_name": {Default: &info.Default{Value: "bridge-generated"}},
			}},
		}
		result := stripStaleDefaults(m, tfs, ps)
		assert.Equal(t, m, result, "nested field with bridge Default must be preserved")
	})

	t.Run("nested field absent from nested schema is stripped", func(t *testing.T) {
		// Inside a MaxItemsOne block, a __defaults entry naming a field that no
		// longer exists in the nested schema must be stripped. Mirrors the
		// top-level "field removed from TF schema" case at the nested level.
		nested := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("removedNested"),
			}),
			"removedNested": resource.NewStringProperty("orphaned-value"),
			"keptNested":    resource.NewStringProperty("user-set"),
		})
		m := resource.PropertyMap{
			"block": nested,
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"block": (&schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						// removed_nested is gone from the schema entirely.
						"kept_nested": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		blockVal := result["block"].ObjectValue()
		_, hasRemoved := blockVal["removedNested"]
		assert.False(t, hasRemoved, "field absent from nested schema must be stripped")
		assert.Equal(t, resource.NewStringProperty("user-set"), blockVal["keptNested"])
	})

	t.Run("stale default in array element nested object is stripped", func(t *testing.T) {
		// TypeList without MaxItems=1 maps to an array of objects in Pulumi.
		// Each element can have its own __defaults.
		elem0 := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("elemStale"),
			}),
			"elemStale": resource.NewStringProperty("old-default"),
			"elemKept":  resource.NewStringProperty("user-set"),
		})
		m := resource.PropertyMap{
			"items": resource.NewArrayProperty([]resource.PropertyValue{elem0}),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"items": (&schema.Schema{
				Type:     shim.TypeList,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"elem_stale": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
						"elem_kept":  (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		items := result["items"].ArrayValue()
		assert.Len(t, items, 1)
		elemVal := items[0].ObjectValue()
		_, hasElemStale := elemVal["elemStale"]
		assert.False(t, hasElemStale, "stale default in array element should be stripped")
		assert.Equal(t, resource.NewStringProperty("user-set"), elemVal["elemKept"])
		_, hasDefaults := elemVal[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "array element __defaults should be removed when empty")
	})

	t.Run("nested __defaults inside TypeSet elements are NOT stripped", func(t *testing.T) {
		// TypeSet membership is hash-based on all element fields. Stripping a field
		// from a nested element changes its hash and makes TF see set reorder/add/remove
		// diffs instead of in-place updates. The recursion must skip TypeSet entirely.
		nestedElem := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("default"),
			}),
			"default":    resource.NewStringProperty("default"),
			"nestedProp": resource.NewStringProperty("val1"),
		})
		m := resource.PropertyMap{
			"items": resource.NewArrayProperty([]resource.PropertyValue{nestedElem}),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"items": (&schema.Schema{
				Type:     shim.TypeSet,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"default": (&schema.Schema{
							Type: shim.TypeString, Optional: true, Default: "default",
						}).Shim(),
						"nested_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		// The TypeSet element must be untouched: the "default" field and the nested
		// __defaults must both still be present, otherwise the set hash changes.
		assert.Equal(t, m, result, "TypeSet element fields and nested __defaults must be preserved")
	})

	t.Run("nested __defaults inside MaxItemsOne TypeSet are NOT stripped", func(t *testing.T) {
		// MaxItemsOne TypeSet flattens to a single object, but TF still hashes it as
		// a set element. Stripping nested fields would change the hash and mislead TF
		// about set membership.
		nested := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("default"),
			}),
			"default":    resource.NewStringProperty("default"),
			"nestedProp": resource.NewStringProperty("val1"),
		})
		m := resource.PropertyMap{
			"item": nested,
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"item": (&schema.Schema{
				Type:     shim.TypeSet,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"default": (&schema.Schema{
							Type: shim.TypeString, Optional: true, Default: "default",
						}).Shim(),
						"nested_prop": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		assert.Equal(t, m, result, "MaxItemsOne TypeSet element fields must be preserved")
	})

	t.Run("stale default in secret-wrapped nested object is stripped", func(t *testing.T) {
		// Nested blocks can be wrapped in MakeSecret(). The recursion must unwrap,
		// strip, and re-wrap.
		inner := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("secretStale"),
			}),
			"secretStale": resource.NewStringProperty("old-default"),
			"secretKept":  resource.NewStringProperty("user-set"),
		})
		m := resource.PropertyMap{
			"block": resource.MakeSecret(inner),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"block": (&schema.Schema{
				Type:      shim.TypeList,
				MaxItems:  1,
				Optional:  true,
				Sensitive: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"secret_stale": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
						"secret_kept":  (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		// Result should still be a secret
		assert.True(t, result["block"].IsSecret())
		blockVal := result["block"].SecretValue().Element.ObjectValue()
		_, hasStale := blockVal["secretStale"]
		assert.False(t, hasStale, "stale default inside secret should be stripped")
		assert.Equal(t, resource.NewStringProperty("user-set"), blockVal["secretKept"])
	})

	t.Run("stale default in secret-wrapped array element is stripped", func(t *testing.T) {
		// Array elements can be individually secret-wrapped. The recursion must
		// unwrap each element, strip stale defaults, and re-wrap.
		elem := resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("elemStale"),
			}),
			"elemStale": resource.NewStringProperty("old-default"),
			"elemKept":  resource.NewStringProperty("user-set"),
		}))
		m := resource.PropertyMap{
			"items": resource.NewArrayProperty([]resource.PropertyValue{elem}),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"items": (&schema.Schema{
				Type:     shim.TypeList,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"elem_stale": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
						"elem_kept":  (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		items := result["items"].ArrayValue()
		assert.Len(t, items, 1)
		assert.True(t, items[0].IsSecret(), "element should still be secret-wrapped")
		elemVal := items[0].SecretValue().Element.ObjectValue()
		_, hasStale := elemVal["elemStale"]
		assert.False(t, hasStale, "stale default in secret-wrapped array element should be stripped")
		assert.Equal(t, resource.NewStringProperty("user-set"), elemVal["elemKept"])
	})

	t.Run("deeply nested stale default is stripped (object -> array -> object)", func(t *testing.T) {
		// Two levels of nesting: top-level block -> array of items -> each item has __defaults
		innerElem := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("deepStale"),
			}),
			"deepStale": resource.NewStringProperty("old-default"),
			"deepKept":  resource.NewStringProperty("user-set"),
		})
		m := resource.PropertyMap{
			"outer": resource.NewObjectProperty(resource.PropertyMap{
				"innerList": resource.NewArrayProperty([]resource.PropertyValue{innerElem}),
			}),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"outer": (&schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"inner_list": (&schema.Schema{
							Type:     shim.TypeList,
							Optional: true,
							Elem: (&schema.Resource{
								Schema: schema.SchemaMap{
									"deep_stale": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
									"deep_kept":  (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
								},
							}).Shim(),
						}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		outerVal := result["outer"].ObjectValue()
		innerList := outerVal["innerList"].ArrayValue()
		assert.Len(t, innerList, 1)
		deepVal := innerList[0].ObjectValue()
		_, hasDeepStale := deepVal["deepStale"]
		assert.False(t, hasDeepStale, "deeply nested stale default should be stripped")
		assert.Equal(t, resource.NewStringProperty("user-set"), deepVal["deepKept"])
	})

	t.Run("top-level stale block with nested stale defaults is stripped, not re-inserted", func(t *testing.T) {
		// Regression: when a top-level key is listed in __defaults (so it's a stale
		// top-level default) AND its value also contains nested __defaults that need
		// stripping, the recursion path must not re-insert the key after the top-level
		// strip removes it.
		nested := resource.NewObjectProperty(resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("nestedStale"),
			}),
			"nestedStale": resource.NewStringProperty("old-nested-default"),
		})
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("block"),
			}),
			"block": nested,
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"block": (&schema.Schema{
				Type:     shim.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: (&schema.Resource{
					Schema: schema.SchemaMap{
						"nested_stale": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
					},
				}).Shim(),
			}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		_, hasBlock := result["block"]
		assert.False(t, hasBlock, "top-level stale block should be stripped, not re-inserted by nested-change handling")
		_, hasDefaults := result[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "__defaults should be removed when empty")
	})

	t.Run("does not mutate original map", func(t *testing.T) {
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("staleField"),
			}),
			"staleField": resource.NewStringProperty("old-default"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"stale_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
		})
		result := stripStaleDefaults(m, tfs, nil)
		// Original should still have the field
		assert.Equal(t, resource.NewStringProperty("old-default"), m["staleField"])
		// Result should not
		_, hasStale := result["staleField"]
		assert.False(t, hasStale)
	})
}
