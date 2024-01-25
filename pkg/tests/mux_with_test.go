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

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func newTFProvider() *schema.Provider {
	resource := &schema.Resource{
		Create: func(rd *schema.ResourceData, i interface{}) (err error) {
			v, ok := rd.GetOk("seed")
			if !ok {
				return fmt.Errorf(`missing parameter "seed"`)
			}
			err = rd.Set("value", v.(int)*10)
			if err != nil {
				return
			}
			rd.SetId("1")
			return
		},
		Schema: map[string]*schema.Schema{
			"seed": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"value": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}

	datasource := &schema.Resource{
		Read: func(rd *schema.ResourceData, i interface{}) (err error) {
			err = rd.Set("number", 10)
			if err != nil {
				return
			}
			rd.SetId("1")
			return
		},
		Schema: map[string]*schema.Schema{
			"number": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}

	return &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"random_number": resource,
		},
		DataSourcesMap: map[string]*schema.Resource{
			"random_number": datasource,
		},
	}
}

func newProviderServer(info tfbridge.ProviderInfo) (server pulumirpc.ResourceProviderServer, err error) {
	ctx := context.Background()
	schema, err := tfgen.GenerateSchema(info, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	server = tfbridge.NewProvider(ctx,
		nil,          /* hostClient */
		info.Name,    /* module */
		info.Version, /* version */
		info.P,       /* tf */
		info,         /* info */
		data,         /* pulumiSchema */
	)
	return
}

func TestMuxWithProvider(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P:          shimv2.NewProvider(newTFProvider()),
		Name:       "random",
		Keywords:   []string{"pulumi", "random"},
		License:    "Apache-2.0",
		Homepage:   "https://pulumi.io",
		Repository: "https://github.com/pulumi/pulumi-random",
		Version:    "0.0.3",
		Resources: map[string]*tfbridge.ResourceInfo{
			"random_number": {
				Tok: "random:index/randomNumber:RandomNumber",
			},
		},
		DataSources: map[string]*tfbridge.DataSourceInfo{
			"random_number": {
				Tok: "random:index/getRandomNumber:getRandomNumber",
			},
		},
	}

	server, err := newProviderServer(info)
	assert.NoError(t, err)

	grpcTestCases := []string{
		`
		{
			"method": "/pulumirpc.ResourceProvider/Create",
			"request": {
				"urn": "urn:pulumi:dev::teststack::random:index/randomNumber:RandomNumber::rn",
				"properties": {
					"__defaults": [],
					"seed": 15
				}
			},
			"response": {
				"id": "1",
				"properties": {
					"id": "1",
					"seed": 15,
					"value": 150
				}
			}
		}
		`,
		`
		{
			"method": "/pulumirpc.ResourceProvider/Invoke",
			"request": {
				"tok": "random:index/getRandomNumber:getRandomNumber"
			},
			"response": {
				"return": {
					"id": "1",
					"number": 10
				}
			}
		}
		`,
	}
	for i := range grpcTestCases {
		testutils.Replay(t, server, grpcTestCases[i])
	}

	info.MetadataInfo = tfbridge.NewProviderMetadata(nil)
	info.MuxWith = []tfbridge.MuxProvider{
		newMuxProvider(),
	}

	server, err = newProviderServer(info)
	assert.NoError(t, err)

	grpcMuxTestCases := []string{
		`
		{
			"method": "/pulumirpc.ResourceProvider/Create",
			"request": {
				"urn": "urn:pulumi:dev::teststack::random:index/randomNumber:RandomNumber::rn",
				"properties": {
					"__defaults": [],
					"seed": 15
				}
			},
			"response": {
				"id": "1",
				"properties": {
					"id": "1",
					"seed": 15,
					"value": 150
				}
			}
		}
		`,
		`
		{
			"method": "/pulumirpc.ResourceProvider/Invoke",
			"request": {
				"tok": "random:index/getRandomNumber:getRandomNumber"
			},
			"response": {
				"return": {
					"number": 20
				}
			}
		}
		`,
	}
	for i := range grpcTestCases {
		testutils.Replay(t, server, grpcMuxTestCases[i])
	}
}

func newMuxProvider() tfbridge.MuxProvider {
	return &tfMuxProvider{
		packageSchema: pschema.PackageSpec{
			Name: "random",
			Functions: map[string]pschema.FunctionSpec{
				"random:index/getRandomNumber:getRandomNumber": {
					ReturnType: &pschema.ReturnTypeSpec{
						ObjectTypeSpec: &pschema.ObjectTypeSpec{
							Type: "object",
							Properties: map[string]pschema.PropertySpec{
								"number": {
									TypeSpec: pschema.TypeSpec{
										Type: "integer",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

type tfMuxProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	packageSchema pschema.PackageSpec
}

func (p *tfMuxProvider) GetSpec(ctx context.Context, name, version string) (pschema.PackageSpec, error) {
	return p.packageSchema, nil
}

func (p *tfMuxProvider) GetInstance(ctx context.Context, name, version string, host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
	return p, nil
}

func (p *tfMuxProvider) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (res *pulumirpc.InvokeResponse, err error) {
	var result *structpb.Struct

	switch req.Tok {
	case "random:index/getRandomNumber:getRandomNumber":
		{
			result, err = plugin.MarshalProperties(
				resource.NewPropertyMapFromMap(map[string]interface{}{
					"number": 20,
				}),
				plugin.MarshalOptions{Label: req.Tok, KeepUnknowns: true, SkipNulls: true},
			)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("tfMuxProvider::Invoke: %q not supported", req.Tok)
	}
	res = &pulumirpc.InvokeResponse{Return: result}
	return
}
