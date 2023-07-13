// Copyright 2016-2023, Pulumi Corporation.
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

package tests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestRegress940(t *testing.T) {
	r := resourceDockerImage()
	shimmedR := shimv2.NewResource(r)

	var config, olds, news resource.PropertyMap
	var instance *tfbridge.PulumiResource

	news = resource.PropertyMap{
		"build": resource.NewObjectProperty(resource.PropertyMap{
			"buildArg": resource.NewObjectProperty(resource.PropertyMap{
				"foo":    resource.NewStringProperty("bar"),
				"":       resource.NewStringProperty("baz"),
				"fooBar": resource.NewStringProperty("foo_bar_value"),
			}),
		}),
	}

	result, _, err := tfbridge.MakeTerraformInputs(instance, config, olds, news, shimmedR.Schema(), map[string]*tfbridge.SchemaInfo{})

	t.Run("no error with empty keys", func(t *testing.T) {
		assert.NoError(t, err)
	})

	t.Run("map keys are not renamed to Pulumi-style names", func(t *testing.T) {
		// buildArg becomes build_arg  because it is an object property
		// in contrast, fooBar stays the same because it is a map key
		// note also that build becomes array-wrapped because of MaxItems=1 flattening
		assert.Equal(t, "foo_bar_value", result["build"].([]any)[0].(map[string]any)["build_arg"].(map[string]any)["fooBar"])
	})
}

func resourceDockerImage() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"build": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"build_arg": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
		},
	}
}
