// Copyright 2016-2019, Pulumi Corporation.
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

package provider

import (
	"fmt"
	"log"

	"github.com/golang/protobuf/ptypes/empty"
	backendInit "github.com/hashicorp/terraform/backend/init"
	"github.com/hashicorp/terraform/svchost/disco"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/provider"
	"github.com/pulumi/pulumi/sdk/proto/go"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-terraform/pkg/tfbridge"
)

type Provider struct {
	version string
}

func NewProvider(ctx context.Context, host *provider.HostClient, version string) *Provider {
	log.SetOutput(tfbridge.NewTerraformLogRedirector(ctx, host))
	backendInit.Init(disco.New())

	p := &Provider{
		version: version,
	}
	return p
}

func validateAndExtractResourceType(urnValue string) (string, error) {
	urn := resource.URN(urnValue)
	resourceType := urn.Type()

	const resourceTypeRemoteStateReference = "terraform:state:RemoteStateReference"

	switch resourceType {
	case resourceTypeRemoteStateReference:
		return resourceTypeRemoteStateReference, nil
	default:
		return "", status.Error(codes.InvalidArgument, fmt.Sprintf("unknown resource type: %q", resourceType))
	}
}

func (*Provider) CheckConfig(context.Context, *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CheckConfig is not yet implemented")
}

func (*Provider) DiffConfig(context.Context, *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")
}

func (p *Provider) Configure(context.Context, *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{}, nil
}

func (*Provider) Invoke(context.Context, *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Invoke is not yet implemented")
}

func (*Provider) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "Check is not yet implementd")
}

func (*Provider) Diff(context.Context, *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Diff is not yet implemented")
}

func (*Provider) Create(context.Context, *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Create is not yet implemented")
}

func (*Provider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	if _, err := validateAndExtractResourceType(req.Urn); err != nil {
		return nil, err
	}

	return remoteStateReferenceRead(ctx, req)
}

func (*Provider) Update(context.Context, *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Update is not yet implemented")
}

func (*Provider) Delete(context.Context, *pulumirpc.DeleteRequest) (*empty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "Delete is not yet implemented")
}

func (*Provider) Cancel(context.Context, *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (p *Provider) GetPluginInfo(context.Context, *empty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}
