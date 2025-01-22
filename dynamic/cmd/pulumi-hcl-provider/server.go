package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type hclResourceProviderServer struct {
	params             ParameterizeArgs // available after Parameterize call only
	hc                 *provider.HostClient
	moduleStateHandler *moduleStateHandler
	pulumirpc.UnimplementedResourceProviderServer
}

var _ pulumirpc.ResourceProviderServer = (*hclResourceProviderServer)(nil)

func newHclResourceProviderServer(hc *provider.HostClient) *hclResourceProviderServer {
	modStateHandler := newModuleStateHandler(hc)
	return &hclResourceProviderServer{
		hc:                 hc,
		moduleStateHandler: modStateHandler,
	}
}

func (s *hclResourceProviderServer) Parameterize(
	ctx context.Context,
	req *pulumirpc.ParameterizeRequest,
) (*pulumirpc.ParameterizeResponse, error) {
	params, err := parseParameterizeRequest(req)
	if err != nil {
		return nil, err
	}
	s.params = params
	return &pulumirpc.ParameterizeResponse{
		Name:    providerName,
		Version: providerVersion,
	}, nil
}

func (s *hclResourceProviderServer) GetSchema(
	ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	if req.Version != 0 {
		return nil, fmt.Errorf("req.Version is not yet supported")
	}
	spec, err := inferPulumiSchemaForModule(&s.params)
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

func (rps *hclResourceProviderServer) construct(
	ctx *pulumi.Context,
	typ, name string,
	inputs pulumiprovider.ConstructInputs,
	options pulumi.ResourceOption,
) (*pulumiprovider.ConstructResult, error) {
	switch typ {
	case "hcl:index:VpcAws":
		component, err := NewModuleComponentResource(ctx, rps.moduleStateHandler, typ, name, &ModuleComponentArgs{})
		if err != nil {
			return nil, fmt.Errorf("NewModuleComponentResource failed: %w", err)
		}
		constructResult, err := pulumiprovider.NewConstructResult(component)
		if err != nil {
			return nil, fmt.Errorf("pulumiprovider.NewConstructResult failed: %w", err)
		}
		return constructResult, nil
	default:
		return nil, fmt.Errorf("TODO: only hcl:index:VpcAws is supported in the prototype")
	}
}

func (rps *hclResourceProviderServer) Construct(
	ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (*pulumirpc.ConstructResponse, error) {
	return pulumiprovider.Construct(ctx, req, rps.hc.EngineConn(), rps.construct)

	// contract.Assertf(req.Type == "hcl:index:VpcAws", "TODO only hcl:index:VpcAws is supported in Construct")

	// componentURN := urnCreate(req.Name, req.Type, urn.URN(req.Parent), req.Project, req.Stack)

	// resmonClient, err := newResourceMonitorClient(req.MonitorEndpoint)
	// contract.AssertNoErrorf(err, "Failed initializing a resource monitor client")

	// go rps.registerModuleStateResource(ctx, resmonClient, componentURN)

	// d, err := prepareTFWorkspace()
	// if err != nil {
	// 	return nil, err
	// }

	// err = initTF(d)
	// if err != nil {
	// 	return nil, err
	// }

	// requiredProviders, err := inferTFRequiredProviders(d)
	// if err != nil {
	// 	return nil, err
	// }

	// proxies, err := startTFProviderProxies(requiredProviders, req.MonitorEndpoint, req.DryRun)
	// if err != nil {
	// 	return nil, err
	// }

	// defer func() {
	// 	err := proxies.Close()
	// 	contract.AssertNoErrorf(err, "failed to close proxies")
	// }()

	// if req.DryRun == true {
	// 	// Handle pulumi preview.
	// 	err = planTF(d, proxies)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// } else {
	// 	// handle pulumi up
	// 	err = upTF(d, proxies)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// return &pulumirpc.ConstructResponse{
	// 	State: &structpb.Struct{
	// 		Fields: map[string]*structpb.Value{
	// 			"defaultVpcId": structpb.NewStringValue("testing"),
	// 		},
	// 	},
	// }, nil
}

func (rps *hclResourceProviderServer) Check(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	switch req.GetType() {
	case moduleStateResourceType:
		return rps.moduleStateHandler.Check(ctx, req)
	default:
		return nil, fmt.Errorf("Type %q is not supported yet", req.GetType())
	}
}

func (rps *hclResourceProviderServer) Diff(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	switch req.GetType() {
	case moduleStateResourceType:
		return rps.moduleStateHandler.Diff(ctx, req)
	default:
		return nil, fmt.Errorf("Type %q is not supported yet", req.GetType())
	}
}

func (rps *hclResourceProviderServer) Create(
	ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	switch req.GetType() {
	case moduleStateResourceType:
		return rps.moduleStateHandler.Create(ctx, req)
	default:
		return nil, fmt.Errorf("Type %q is not supported yet", req.GetType())
	}
}

func (rps *hclResourceProviderServer) Update(
	ctx context.Context,
	req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	switch req.GetType() {
	case moduleStateResourceType:
		return rps.moduleStateHandler.Update(ctx, req)
	default:
		return nil, fmt.Errorf("Type %q is not supported yet", req.GetType())
	}
}

func (rps *hclResourceProviderServer) Delete(
	ctx context.Context,
	req *pulumirpc.DeleteRequest,
) (*emptypb.Empty, error) {
	switch req.GetType() {
	case moduleStateResourceType:
		return rps.moduleStateHandler.Delete(ctx, req)
	default:
		return nil, fmt.Errorf("Type %q is not supported yet", req.GetType())
	}
}
