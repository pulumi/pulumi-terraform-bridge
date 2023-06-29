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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestRegressAws2490(t *testing.T) {
	ctx := context.Background()

	resourceIntegrationCreate := func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		var diags diag.Diagnostics
		conn := meta.(*conns.AWSClient).APIGatewayV2Conn()

		req := &apigatewayv2.CreateIntegrationInput{
			ApiId:           aws.String(d.Get("api_id").(string)),
			IntegrationType: aws.String(d.Get("integration_type").(string)),
		}
		if v, ok := d.GetOk("connection_id"); ok {
			req.ConnectionId = aws.String(v.(string))
		}
		if v, ok := d.GetOk("connection_type"); ok {
			req.ConnectionType = aws.String(v.(string))
		}
		if v, ok := d.GetOk("content_handling_strategy"); ok {
			req.ContentHandlingStrategy = aws.String(v.(string))
		}
		if v, ok := d.GetOk("credentials_arn"); ok {
			req.CredentialsArn = aws.String(v.(string))
		}
		if v, ok := d.GetOk("description"); ok {
			req.Description = aws.String(v.(string))
		}
		if v, ok := d.GetOk("integration_method"); ok {
			req.IntegrationMethod = aws.String(v.(string))
		}
		if v, ok := d.GetOk("integration_subtype"); ok {
			req.IntegrationSubtype = aws.String(v.(string))
		}
		if v, ok := d.GetOk("integration_uri"); ok {
			req.IntegrationUri = aws.String(v.(string))
		}
		if v, ok := d.GetOk("passthrough_behavior"); ok {
			req.PassthroughBehavior = aws.String(v.(string))
		}
		if v, ok := d.GetOk("payload_format_version"); ok {
			req.PayloadFormatVersion = aws.String(v.(string))
		}
		if v, ok := d.GetOk("request_parameters"); ok {
			req.RequestParameters = flex.ExpandStringMap(v.(map[string]interface{}))
		}
		if v, ok := d.GetOk("request_templates"); ok {
			req.RequestTemplates = flex.ExpandStringMap(v.(map[string]interface{}))
		}
		if v, ok := d.GetOk("response_parameters"); ok && v.(*schema.Set).Len() > 0 {
			req.ResponseParameters = expandIntegrationResponseParameters(v.(*schema.Set).List())
		}
		if v, ok := d.GetOk("template_selection_expression"); ok {
			req.TemplateSelectionExpression = aws.String(v.(string))
		}
		if v, ok := d.GetOk("timeout_milliseconds"); ok {
			req.TimeoutInMillis = aws.Int64(int64(v.(int)))
		}
		if v, ok := d.GetOk("tls_config"); ok {
			req.TlsConfig = expandTLSConfig(v.([]interface{}))
		}

		resp, err := conn.CreateIntegrationWithContext(ctx, req)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "creating API Gateway v2 integration: %s", err)
		}

		d.SetId("some-id")
		return append(diags, resourceIntegrationRead(ctx, d, meta)...)
	}

	resource := &schema.Resource{
		CreateWithoutTimeout: resourceIntegrationCreate,
		Schema: map[string]*schema.Schema{
			"passthrough_behavior": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "WHEN_NO_MATCH",
				// ValidateFunc: validation.StringInSlice(apigatewayv2.PassthroughBehavior_Values(), false),
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					// Not set for HTTP APIs.
					if old == "" && new == "WHEN_NO_MATCH" {
						return true
					}
					return false
				},
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"aws_apigateway2_integration": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider) // , shimv2.WithDiffStrategy(shimv2.PlanState))

	info := tfbridge.ProviderInfo{
		P:           p,
		Name:        "aws",
		Description: "A Pulumi package for creating and managing Amazon Web Services (AWS) cloud resources.",
		Keywords:    []string{"pulumi", "aws"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/phillipedwards/pulumi-aws",
		Version:     "0.0.2",
		Resources: map[string]*tfbridge.ResourceInfo{
			"aws_apigateway2_integration": {Tok: "aws:apigatewayv2/integration:Integration"},
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
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::aws-2490::aws:apigatewayv2/integration:Integration::example",
	    "properties": {
	      "__defaults": [
		"connectionType"
	      ],
	      "passthroughBehavior": "NEVER",
	    }
	  },
	  "response": {
	    "id": "qkmc19h",
	    "properties": {
	      "passthroughBehavior": "",
	    }
	  }
	}`
	testutils.Replay(t, server, testCase)
}
