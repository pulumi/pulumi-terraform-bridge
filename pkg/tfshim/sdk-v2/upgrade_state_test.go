package sdkv2

import (
	"context"
	// "math/big"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeResourceState(t *testing.T) {
	state := func() *terraform.InstanceState {
		n, err := cty.ParseNumberVal("641577219598130723")
		require.NoError(t, err)
		v := cty.ObjectVal(map[string]cty.Value{"x": n})
		s := terraform.NewInstanceStateShimmedFromValue(v, 0)
		s.Meta["schema_version"] = "0"
		s.ID = "id"
		s.RawState = v
		s.Attributes["id"] = s.ID
		return s
	}

	rSchema := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"x": {Type: schema.TypeInt, Optional: true},
		},
	}

	require.NoError(t, rSchema.InternalValidate(rSchema.Schema, true))

	t.Logf(`Attributes["x"]: %s`, state().Attributes["x"])

	const tfToken = "test_token"

	actual, err := upgradeResourceState(context.Background(), tfToken, &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			tfToken: {
				UseJSONNumber: true,
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
		},
	}, rSchema, state())

	require.NoError(t, err)
	assert.Equal(t, state().Attributes, actual.Attributes)
}
