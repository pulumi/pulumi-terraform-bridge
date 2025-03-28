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

package sdkv2

import (
	"context"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestRawPlanSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
	p := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{"myres": r},
	}

	wp := NewProvider(p)

	state := cty.ObjectVal(map[string]cty.Value{
		"tags": cty.MapVal(map[string]cty.Value{"tag1": cty.StringVal("tag1v")}),
	})

	config := cty.ObjectVal(map[string]cty.Value{
		"tags": cty.MapVal(map[string]cty.Value{"tag1": cty.StringVal("tag1v")}),
	})

	instanceState := terraform.NewInstanceStateShimmedFromValue(state, 0)
	instanceState.ID = "oldid"
	instanceState.Meta = map[string]interface{}{} // ignore schema versions for this test
	resourceConfig := terraform.NewResourceConfigShimmed(config, r.CoreConfigSchema())

	ss := v2InstanceState2{
		resourceType: "myres",
		stateValue:   state,
	}

	id, err := wp.Diff(ctx, "myres", &ss, v2ResourceConfig{
		tf: resourceConfig,
	}, shim.DiffOptions{})
	require.NoError(t, err)

	assert.False(t, id.(*v2InstanceDiff2).tf.RawPlan.IsNull(), "RawPlan should not be Null")
}
