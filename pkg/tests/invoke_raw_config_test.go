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
	"context"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	testutils "github.com/pulumi/providertest/replay"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func getRawConfigAttr(raw cty.Value, key string) (cty.Value, bool) {
	// GetRawConfig can omit attributes from the object type entirely when the
	// user left them out, so cross-tests need a safe probe instead of GetAttr.
	if !raw.Type().IsObjectType() || !raw.Type().HasAttribute(key) {
		return cty.NilVal, false
	}
	v := raw.GetAttr(key)
	if v.IsNull() {
		return cty.NilVal, false
	}
	return v, true
}

func TestInvokeRawConfigDoesNotPanic(t *testing.T) {
	t.Parallel()

	resource := &schema.Resource{
		ReadWithoutTimeout: func(ctx context.Context, data *schema.ResourceData, i interface{}) diag.Diagnostics {
			rawConfigVal := data.GetRawConfig().GetAttr("engine")
			err := data.Set("engine", rawConfigVal.AsString())
			if err != nil {
				panic(err)
			}
			data.SetId("123")
			return diag.Diagnostics{}
		},
		Schema: map[string]*schema.Schema{
			"engine": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		DataSourcesMap: map[string]*schema.Resource{
			"aws_rds_engine_version": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider)

	info := tfbridge.ProviderInfo{
		P:    p,
		Name: "aws",
		DataSources: map[string]*tfbridge.DataSourceInfo{
			"aws_rds_engine_version": {Tok: "aws:rds/getEngineVersion:getEngineVersion"},
		},
	}

	server := tfbridge.NewProvider(context.Background(),
		nil,      /* hostClient */
		"aws",    /* module */
		"",       /* version */
		p,        /* tf */
		info,     /* info */
		[]byte{}, /* pulumiSchema */
	)

	testCase := `
	{
  "method": "/pulumirpc.ResourceProvider/Invoke",
  "request": {
    "tok": "aws:rds/getEngineVersion:getEngineVersion",
    "args": {
      "engine": "postgres"
    }
  },
  "response": {
    "return": {
      "engine": "postgres",
	  "id": "123"
    }
  },
  "metadata": {
    "kind": "resource",
    "mode": "client",
    "name": "aws"
  }
}`
	testutils.Replay(t, server, testCase)
}

func TestInvokeCrossTest(t *testing.T) {
	t.Parallel()

	runTest := func(t *testing.T, extraSchema map[string]*schema.Schema, tfBody string, argsMap map[string]interface{}) {
		var tfRd *schema.ResourceData
		var puRd *schema.ResourceData

		resourceSchema := map[string]*schema.Schema{
			"engine": {
				Type:     schema.TypeString,
				Required: true,
			},
		}
		for k, v := range extraSchema {
			resourceSchema[k] = v
		}

		resource := &schema.Resource{
			ReadWithoutTimeout: func(ctx context.Context, data *schema.ResourceData, _ interface{}) diag.Diagnostics {
				if tfRd == nil {
					tfRd = data
				} else {
					puRd = data
				}

				if v, ok := getRawConfigAttr(data.GetRawConfig(), "is_signup_enabled"); ok {
					err := data.Set("is_signup_enabled", v.True())
					require.NoError(t, err)
				}
				if v, ok := getRawConfigAttr(data.GetRawConfig(), "default_location"); ok {
					err := data.Set("default_location", v.AsString())
					require.NoError(t, err)
				}
				rawEngine := data.GetRawConfig().GetAttr("engine")
				err := data.Set("engine", rawEngine.AsString())
				require.NoError(t, err)
				data.SetId("123")
				return nil
			},
			Schema: resourceSchema,
		}

		tfProvider := &schema.Provider{
			Schema: map[string]*schema.Schema{},
			DataSourcesMap: map[string]*schema.Resource{
				"aws_rds_engine_version": resource,
			},
		}

		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "aws", tfcheck.NewTFDriverOpts{SDKProvider: tfProvider})
		tfdriver.Write(t, `
data "aws_rds_engine_version" "test" {
  engine = "postgres"
`+tfBody+`
}
`)
		_, err := tfdriver.Plan(t)
		require.NoError(t, err)
		require.NotNil(t, tfRd)
		require.Nil(t, puRd)

		p := shimv2.NewProvider(tfProvider)
		info := tfbridge.ProviderInfo{
			P:    p,
			Name: "aws",
			DataSources: map[string]*tfbridge.DataSourceInfo{
				"aws_rds_engine_version": {Tok: "aws:rds/getEngineVersion:getEngineVersion"},
			},
		}

		server := tfbridge.NewProvider(context.Background(),
			nil,      /* hostClient */
			"aws",    /* module */
			"",       /* version */
			p,        /* tf */
			info,     /* info */
			[]byte{}, /* pulumiSchema */
		)

		args, err := structpb.NewStruct(argsMap)
		require.NoError(t, err)

		resp, err := server.Invoke(context.Background(), &pulumirpc.InvokeRequest{
			Tok:  "aws:rds/getEngineVersion:getEngineVersion",
			Args: args,
		})
		require.NoError(t, err)
		require.Empty(t, resp.Failures)
		require.NotNil(t, puRd)
		// Data source reads can inspect RawConfig too, so invoke needs the same
		// omission-preserving behavior as resource CRUD and provider Configure.
		require.Equal(t, tfRd.GetRawConfig(), puRd.GetRawConfig())
	}

	t.Run("omitted bool default false", func(t *testing.T) {
		// Matches the auth0-style reported case, but on the Invoke path.
		runTest(t,
			map[string]*schema.Schema{
				"is_signup_enabled": {
					Type:     schema.TypeBool,
					Optional: true,
					Default:  false,
				},
			},
			"",
			map[string]interface{}{
				"engine": "postgres",
			},
		)
	})

	t.Run("omitted string default empty", func(t *testing.T) {
		// Confirms top-level empty-string defaults are also a real RawConfig parity
		// gap, not just a replay-test compatibility artifact.
		runTest(t,
			map[string]*schema.Schema{
				"default_location": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "",
				},
			},
			"",
			map[string]interface{}{
				"engine": "postgres",
			},
		)
	})
}
