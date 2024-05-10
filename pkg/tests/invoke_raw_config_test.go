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
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestInvokeRawConfigDoesNotPanic(t *testing.T) {
	ctx := context.Background()

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

	server := tfbridge.NewProvider(ctx,
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
