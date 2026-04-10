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
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestStripStaleDefaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
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
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
		assert.Equal(t, m, result)
	})

	t.Run("stale default stripped when TF schema has no default", func(t *testing.T) {
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
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
		_, hasStale := result["staleField"]
		assert.False(t, hasStale, "stale default should be stripped")
		assert.Equal(t, resource.NewStringProperty("keep"), result["otherField"])
		_, hasDefaults := result[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "__defaults should be removed when empty")
	})

	t.Run("field with TF Default kept", func(t *testing.T) {
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("activeDefault"),
			}),
			"activeDefault": resource.NewStringProperty("default-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"active_default": (&schema.Schema{
				Type:     shim.TypeString,
				Optional: true,
				Default:  "default-value",
			}).Shim(),
		})
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
		assert.Equal(t, m, result)
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
		result := stripStaleDefaults(ctx, m, nil, tfs, ps)
		assert.Equal(t, m, result)
	})

	t.Run("field not in TF schema kept", func(t *testing.T) {
		// When tfSchema is nil (field removed from schema), keep the field.
		// MakeTerraformConfig will drop it during Pulumi-to-TF conversion anyway.
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("removedField"),
			}),
			"removedField": resource.NewStringProperty("old-value"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{})
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
		assert.Equal(t, m, result)
	})

	t.Run("mixed defaults some stale some active", func(t *testing.T) {
		m := resource.PropertyMap{
			reservedkeys.Defaults: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("staleField"),
				resource.NewStringProperty("activeField"),
			}),
			"staleField":  resource.NewStringProperty("old-default"),
			"activeField": resource.NewStringProperty("current-default"),
		}
		tfs := makeSchemaMap(map[string]shim.Schema{
			"stale_field": (&schema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
			"active_field": (&schema.Schema{
				Type:     shim.TypeString,
				Optional: true,
				Default:  "current-default",
			}).Shim(),
		})
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
		_, hasStale := result["staleField"]
		assert.False(t, hasStale, "stale field should be stripped")
		assert.Equal(t, resource.NewStringProperty("current-default"), result["activeField"])
		// __defaults should only contain the active field
		defaults := result[reservedkeys.Defaults].ArrayValue()
		assert.Len(t, defaults, 1)
		assert.Equal(t, "activeField", defaults[0].StringValue())
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
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
		_, hasDefaults := result[reservedkeys.Defaults]
		assert.False(t, hasDefaults, "__defaults should be removed entirely")
		_, hasStale1 := result["stale1"]
		_, hasStale2 := result["stale2"]
		assert.False(t, hasStale1)
		assert.False(t, hasStale2)
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
		result := stripStaleDefaults(ctx, m, nil, tfs, nil)
		// Original should still have the field
		assert.Equal(t, resource.NewStringProperty("old-default"), m["staleField"])
		// Result should not
		_, hasStale := result["staleField"]
		assert.False(t, hasStale)
	})
}
