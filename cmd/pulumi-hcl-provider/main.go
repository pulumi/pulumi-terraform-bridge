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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func main() {
	err := provider.Main("hcl", func(hc *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return &hclResourceProviderServer{}, nil
	})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}

type hclResourceProviderServer struct {
	pulumirpc.UnimplementedResourceProviderServer
}

func (s *hclResourceProviderServer) Parameterize(
	ctx context.Context,
	req *pulumirpc.ParameterizeRequest,
) (*pulumirpc.ParameterizeResponse, error) {
	if args := req.GetArgs(); args != nil {
		pargs, err := parseParameterizeArgs(args)
		if err != nil {
			return nil, err
		}
		// The name and version values returned here are passed to GetSchema SubpackageName, SubpackageVersion.
		return &pulumirpc.ParameterizeResponse{
			Name:    fmt.Sprintf("hcl::%s", pargs.TFModuleRef),
			Version: string(pargs.TFModuleVersion),
		}, nil
	}
	if args := req.GetValue(); args != nil {
		return nil, fmt.Errorf("value is not yet supported")
	}

	return nil, fmt.Errorf("Impossible")
}

func (s *hclResourceProviderServer) GetSchema(
	ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	if req.Version != 0 {
		return nil, fmt.Errorf("req.Version is not yet supported")
	}
	m := TFModuleRef(strings.TrimPrefix(req.SubpackageName, "hcl::"))
	v := TFModuleVersion(req.SubpackageVersion)
	spec, err := inferPulumiSchemaForModule(m, v)
	if err != nil {
		return nil, err
	}
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal failure over Pulumi Package schema: %w", err)
	}
	return &pulumirpc.GetSchemaResponse{Schema: string(specBytes)}, nil
}

var _ pulumirpc.ResourceProviderServer = (*hclResourceProviderServer)(nil)

// Reference to a Terraform module, for example "terraform-aws-modules/vpc/aws".
type TFModuleRef string

// Version specification for a Terraform module, for example "5.16.0".
type TFModuleVersion string

type ParameterizeArgs struct {
	TFModuleRef     TFModuleRef
	TFModuleVersion TFModuleVersion
}

func parseParameterizeArgs(args *pulumirpc.ParameterizeRequest_ParametersArgs) (ParameterizeArgs, error) {
	if len(args.Args) != 2 {
		return ParameterizeArgs{}, fmt.Errorf("Expected exactly 2 args")
	}
	return ParameterizeArgs{
		TFModuleRef:     TFModuleRef(args.Args[0]),
		TFModuleVersion: TFModuleVersion(args.Args[1]),
	}, nil
}

func inferPulumiSchemaForModule(mref TFModuleRef, mver TFModuleVersion) (*schema.PackageSpec, error) {
	if mref == "terraform-aws-modules/vpc/aws" && mver == "5.16.0" {
		return &schema.PackageSpec{
			Name:    "vpcaws",
			Version: string(mver),
			Resources: map[string]schema.ResourceSpec{
				"vpcaws:index:VpcAws": {
					InputProperties: map[string]schema.PropertySpec{
						"cidr": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"defaultVpcId": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					IsComponent: true,
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("Cannot infer Pulumi PackageSpec for TF module %q at version %q", mref, mver)
}
