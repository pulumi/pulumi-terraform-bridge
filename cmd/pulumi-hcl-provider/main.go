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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
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

func (*hclResourceProviderServer) GetPluginInfo(
	ctx context.Context,
	req *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func (*hclResourceProviderServer) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptOutputs:   true,
		AcceptResources: true,
	}, nil
}

func (*hclResourceProviderServer) Construct(
	ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (*pulumirpc.ConstructResponse, error) {
	contract.Assertf(req.Type == "hcl:index:VpcAws", "TODO only hcl:index:VpcAws is supported in Construct")
	contract.Assertf(req.DryRun == true, "TODO Construct only works in preview for now")

	d, err := prepareTFWorkspace()
	if err != nil {
		return nil, err
	}

	err = initTF(d)
	if err != nil {
		return nil, err
	}

	return &pulumirpc.ConstructResponse{
		State: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"defaultVpcId": structpb.NewStringValue("testing"),
			},
		},
	}, nil
}

// Runs terraform init in a given d directory.
//
// Running terraform init will:
//
//	resolve and download modules to .terraform/modules
//	resolve and download providers to .terraform/providers
//	build .terraform.lock.hcl with resolved versions
//
// For the purposes of this code provider binaries will not be needed, so there is is a bit inefficient.
func initTF(d string) error {
	cmd := exec.Command("terraform", "init")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Dir = d
	return cmd.Run()
}

// Prepare a folder with TF files to send
func prepareTFWorkspace() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	d := filepath.Join(wd, ".hcl", "temp-workspace")

	err = os.RemoveAll(d)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(d, 0755)
	if err != nil {
		return "", err
	}

	// TODO inputs to TF need to be translated from Pulumi inputs.
	//
	// https://developer.hashicorp.com/terraform/language/syntax/json
	jsonTF := map[string]any{
		"module": map[string]any{
			"vpc": map[string]any{
				"source":             "terraform-aws-modules/vpc/aws",
				"name":               "by-vpc",
				"cidr":               "10.0.0.0/16",
				"azs":                []any{"us-west-2a", "es-west-2b"},
				"private_subnets":    []any{"10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"},
				"public_subnets":     []any{"10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"},
				"enable_nat_gateway": true,
				"enable_vpn_gateway": true,
			},
		},
	}

	jsonTFBytes, err := json.MarshalIndent(jsonTF, "", "  ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filepath.Join(d, "infra.tf.json"), jsonTFBytes, 0755)
	if err != nil {
		return "", err
	}
	return d, nil
}

// OK so.. We build a TF workspace;
// we infer which providers are needed
// we use debug functionality to make these providers.
// then we run terraform plan

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
			Name:    "hcl",
			Version: "0.0.1",
			Resources: map[string]schema.ResourceSpec{
				"hcl:index:VpcAws": {
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
