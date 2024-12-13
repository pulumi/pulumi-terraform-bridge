package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	bridgedAwsProvider "github.com/pulumi/pulumi-aws/provider/v6"
	"github.com/pulumi/pulumi-aws/provider/v6/pkg/version"
	"github.com/pulumi/pulumi-terraform-bridge/dynamic/internal/shim/run"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	status "google.golang.org/grpc/status"
)

func loadAWSProvider(ctx context.Context) (run.Provider, error) {
	return run.NamedProvider(ctx, "hashicorp/aws", "5.80.0")
}

func newResourceMonitorClient(monitorEndpoint string) (pulumirpc.ResourceMonitorClient, error) {
	// Connect to the resource monitor and create an appropriate client.
	conn, err := grpc.NewClient(
		monitorEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to resource monitor: %w", err)
	}
	return pulumirpc.NewResourceMonitorClient(conn), nil
}

func newTfProxyProviderServer(monitorEndoint string, dryRun bool) *tfProxyProviderServer {
	version.Version = "6.64.0"
	bridged := bridgedAwsProvider.Provider()
	c, err := newResourceMonitorClient(monitorEndoint)
	contract.AssertNoErrorf(err, "loading AWS provider failed")
	awsProvider, err := loadAWSProvider(context.Background())
	contract.AssertNoErrorf(err, "loading AWS provider failed")
	return &tfProxyProviderServer{
		monitorClient: c,
		awsProvider:   awsProvider,
		awsBridged:    bridged,
		dryRun:        dryRun,
	}
}

type tfProxyProviderServer struct {
	monitorClient pulumirpc.ResourceMonitorClient
	awsProvider   run.Provider
	awsBridged    *info.Provider
	UnimplementedProviderServer
	resourceSchemas map[string]*tfprotov6.Schema
	dryRun          bool
}

func (p *tfProxyProviderServer) GetMetadata(
	ctx context.Context,
	req *tfprotov6.GetMetadataRequest,
) (*tfprotov6.GetMetadataResponse, error) {
	return p.awsProvider.GetMetadata(ctx, req)
}

func (p *tfProxyProviderServer) GetProviderSchema(
	ctx context.Context,
	req *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	resp, err := p.awsProvider.GetProviderSchema(ctx, req)
	if err != nil {
		return resp, err
	}
	p.resourceSchemas = resp.ResourceSchemas
	return resp, nil
}

func (p *tfProxyProviderServer) ValidateDataResourceConfig(
	ctx context.Context,
	req *tfprotov6.ValidateDataResourceConfigRequest,
) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return p.awsProvider.ValidateDataResourceConfig(ctx, req)
}

func (p *tfProxyProviderServer) ValidateResourceConfig(
	ctx context.Context,
	req *tfprotov6.ValidateResourceConfigRequest,
) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return p.awsProvider.ValidateResourceConfig(ctx, req)
}

func (p *tfProxyProviderServer) ValidateProviderConfig(
	ctx context.Context,
	req *tfprotov6.ValidateProviderConfigRequest,
) (*tfprotov6.ValidateProviderConfigResponse, error) {
	return p.awsProvider.ValidateProviderConfig(ctx, req)
}

func (p *tfProxyProviderServer) ConfigureProvider(
	ctx context.Context,
	req *tfprotov6.ConfigureProviderRequest,
) (*tfprotov6.ConfigureProviderResponse, error) {
	return p.awsProvider.ConfigureProvider(ctx, req)
}

func (p *tfProxyProviderServer) PlanResourceChange(
	ctx context.Context,
	req *tfprotov6.PlanResourceChangeRequest,
) (*tfprotov6.PlanResourceChangeResponse, error) {
	priorStateIsNull, err := req.PriorState.IsNull()
	contract.AssertNoErrorf(err, "PriorState.IsNull() should not fail")
	contract.Assertf(priorStateIsNull, "PriorState should be IsNull")
	contract.Assertf(req.PriorPrivate == nil, "PriorPrivate should be nil")

	fmt.Printf("[%s] PlanResourceChange\n", req.TypeName)
	defer fmt.Printf("[%s] PlanResourceChange DONE\n", req.TypeName)

	// TODO this is a temporary code path.
	//
	// The real code should not be calling into the actual awsProvider but instead interpreting the results of
	// RegisterResource that will inside them the results of planning the change.
	resp, err := p.awsProvider.PlanResourceChange(ctx, req)
	if err != nil {
		return nil, err
	}

	rn := tfResourceName(req.TypeName)

	// Should this translate req.Config or req.ProposedNewState? In case the state is nil these are usually the
	// same. The underlying bridged provider will re-plan the ProposedNewState anyway. Going with Config.
	obj, err := translateResourceArgs(ctx, rn, req.Config, p.resourceSchemas, p.awsBridged, req.TypeName)
	if err != nil {
		return nil, err
	}

	// In dryRun=true this runs under `pulumi preview` and `terraform plan` and needs to interact with Pulumi.
	//
	// In dryRun=false this runs under `pulumi up` and `terraform apply`. In this case Terraform plan may contain
	// unknowns but Pulumi will no longer tolerate unknowns. Simply ignore Pulumi in this case as it will catch up
	// and do the right thing during ApplyResourceChange.
	if p.dryRun {
		_, err = p.monitorClient.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:            translateTypeName(p.awsBridged, rn),
			Name:            translateResourceName(req.Config),
			Custom:          true,
			Object:          obj,
			AcceptSecrets:   true,
			AcceptResources: true,
		})
	}

	// TODO Parent: this should be parented on the component presumably?
	return resp, err
}

func (p *tfProxyProviderServer) ApplyResourceChange(
	ctx context.Context,
	req *tfprotov6.ApplyResourceChangeRequest,
) (*tfprotov6.ApplyResourceChangeResponse, error) {
	priorStateIsNull, err := req.PriorState.IsNull()
	contract.AssertNoErrorf(err, "PriorState.IsNull() should not fail")
	contract.Assertf(priorStateIsNull, "PriorState should be IsNull")

	planningDelete, err := req.PlannedState.IsNull()
	contract.AssertNoErrorf(err, "req.PlannedState.IsNull() should not fail")
	if planningDelete {
		// Pulumi will take care of deleting out of band.
		//
		// Need to pretend to TF that we deleted the resource successfully.
		return &tfprotov6.ApplyResourceChangeResponse{
			NewState: req.PlannedState, // that is, return null
		}, nil
	}

	fmt.Printf("[%s] ApplyResourceChange\n", req.TypeName)
	defer fmt.Printf("[%s] ApplyResourceChange DONE\n", req.TypeName)

	rn := tfResourceName(req.TypeName)

	// Discard req.PlannedState here because Pulumi will re-plan with the proper state. Use Config as it is supposed
	// to represent the program being interpreted.
	obj, err := translateResourceArgs(ctx, rn, req.Config, p.resourceSchemas, p.awsBridged, req.TypeName)
	if err != nil {
		return nil, err
	}

	resp, err := p.monitorClient.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:            translateTypeName(p.awsBridged, rn),
		Name:            translateResourceName(req.Config),
		Custom:          true,
		Object:          obj,
		AcceptSecrets:   true,
		AcceptResources: true,
	})
	if err != nil {
		return nil, err
	}

	if resp.Result != pulumirpc.Result_SUCCESS {
		return &tfprotov6.ApplyResourceChangeResponse{
			Diagnostics: []*tfprotov6.Diagnostic{{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  fmt.Sprintf("Resource did not provision under Pulumi: %q", resp.Result),
			}},
		}, nil
	}

	// TODO do we need to repackage resp.Id in a special way for Terraform?

	newState, err := translateResourceOutputs(rn, resp.Object, p.resourceSchemas, p.awsBridged, req.TypeName)
	if err != nil {
		return nil, err
	}

	return &tfprotov6.ApplyResourceChangeResponse{
		// Currently without this we cannot pass the objchange.AssertObjectCompatible check even for a simple
		// create scenario.
		//
		// https://github.com/opentofu/opentofu/blob/aeaeacc94d8b5fad8792a8fff273857594085599/internal/tofu/node_resource_abstract_instance.go#L2456

		UnsafeToUseLegacyTypeSystem: true,
		NewState:                    newState,

		// Does the resource private state need to be smuggled back to TF? Probably the answer is yes. The
		// private state is encoded inside Pulumi __meta state and this can be recovered.
		//
		// Private: []byte{},

	}, nil
}

var _ tfprotov6.ProviderServer = (*tfProxyProviderServer)(nil)

type tfProviderProxyHandle struct {
	io.Closer
	ProviderName   string
	ReattachConfig *plugin.ReattachConfig
}

// Serializable version of plugin.ReattachConfig.
func (h *tfProviderProxyHandle) computeReattachConfig() map[string]any {
	return map[string]any{
		"Protocol":        string(h.ReattachConfig.Protocol),
		"ProtocolVersion": h.ReattachConfig.ProtocolVersion,
		"Pid":             os.Getpid(),
		"Test":            h.ReattachConfig.Test,
		"Addr": map[string]any{
			"Network": h.ReattachConfig.Addr.Network(),
			"String":  h.ReattachConfig.Addr.String(),
		},
	}
}

const (
	// See tf6server.envTfReattachProviders, inline since it is internal.
	envTfReattachProviders = "TF_REATTACH_PROVIDERS"
	// See tf6server.grpcMaxMessageSize, inline since it is internal.
	grpcMaxMessageSize = 256 << 20
)

func startTFProviderProxy(providerName, monitorEndpoint string, dryRun bool) (*tfProviderProxyHandle, error) {
	serverHandle, reattachConfig, err := simpleServe(providerName, func() tfprotov6.ProviderServer {
		return newTfProxyProviderServer(monitorEndpoint, dryRun)
	})
	if err != nil {
		return nil, err
	}
	return &tfProviderProxyHandle{
		Closer:         serverHandle,
		ProviderName:   providerName,
		ReattachConfig: reattachConfig,
	}, nil
}

// Inline version of tf6server.ServeConfig
type simpleServeConfig struct {
	//logger       hclog.Logger
	debugCtx     context.Context
	debugCh      chan *plugin.ReattachConfig
	debugCloseCh chan struct{}

	managedDebug                      bool
	managedDebugReattachConfigTimeout time.Duration
	managedDebugStopSignals           []os.Signal

	disableLogInitStderr bool
	disableLogLocation   bool
	useLoggingSink       testing.T
	envVar               string
}

// Modified version of tf6server.Serve
func simpleServe(
	name string,
	serverFactory func() tfprotov6.ProviderServer,
) (io.Closer, *plugin.ReattachConfig, error) {
	opts := []tf6server.ServeOpt{}

	// Defaults
	conf := simpleServeConfig{
		managedDebug:                      true, // need to set this explicitly in the modified version
		managedDebugReattachConfigTimeout: 120 * time.Second,
		managedDebugStopSignals:           []os.Signal{os.Interrupt},
	}

	// Since the ServerOpt struct got inlined this is not working yet:
	//
	// for _, opt := range opts {
	// 	err := opt.ApplyServeOpt(&conf)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	serveConfig := &plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  6,
			MagicCookieKey:   "TF_PLUGIN_MAGIC_COOKIE",
			MagicCookieValue: "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2",
		},
		Plugins: plugin.PluginSet{
			"provider": &tf6server.GRPCProviderPlugin{
				GRPCProvider: serverFactory,
				Opts:         opts,
				Name:         name,
			},
		},
		GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
			opts = append(opts, grpc.MaxRecvMsgSize(grpcMaxMessageSize))
			opts = append(opts, grpc.MaxSendMsgSize(grpcMaxMessageSize))

			return grpc.NewServer(opts...)
		},
	}

	// Disabled to simplify for now:
	//
	// if conf.logger != nil {
	// 	serveConfig.Logger = conf.logger
	// }

	closer := &handleCloser{debugCloseCh: conf.debugCloseCh}

	if conf.managedDebug {
		ctx, cancel := context.WithCancel(context.Background())
		signalCh := make(chan os.Signal, len(conf.managedDebugStopSignals))

		signal.Notify(signalCh, conf.managedDebugStopSignals...)

		closer.onClose = func() {
			signal.Stop(signalCh)
			cancel()
		}

		go func() {
			select {
			case <-signalCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		conf.debugCh = make(chan *plugin.ReattachConfig)
		conf.debugCloseCh = make(chan struct{})
		conf.debugCtx = ctx
	}

	if conf.debugCh != nil {
		serveConfig.Test = &plugin.ServeTestConfig{
			Context:          conf.debugCtx,
			ReattachConfigCh: conf.debugCh,
			CloseCh:          conf.debugCloseCh,
		}
	}

	if !conf.managedDebug {
		plugin.Serve(serveConfig)
		return nil, nil, nil
	}

	go plugin.Serve(serveConfig)

	var pluginReattachConfig *plugin.ReattachConfig

	select {
	case pluginReattachConfig = <-conf.debugCh:
	case <-time.After(conf.managedDebugReattachConfigTimeout):
		return nil, nil, errors.New("timeout waiting on reattach configuration")
	}

	if pluginReattachConfig == nil {
		return nil, nil, errors.New("nil reattach configuration received")
	}

	fmt.Println("SERVING received reattach config")

	return closer, pluginReattachConfig, nil
}

// Helper for [simpleServe].
type handleCloser struct {
	debugCloseCh chan struct{}
	onClose      func()
}

func (hc *handleCloser) Close() error {
	// This does not gracefully wait for the plugin server to be done, but instead assumes the hosting process will shutdown.
	hc.onClose()
	return nil
}

type UnimplementedProviderServer struct{}

func (UnimplementedProviderServer) GetMetadata(context.Context, *tfprotov6.GetMetadataRequest) (*tfprotov6.GetMetadataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMetadata not implemented")
}

func (UnimplementedProviderServer) GetProviderSchema(context.Context, *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProviderSchema not implemented")
}

func (UnimplementedProviderServer) ValidateProviderConfig(context.Context, *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateProviderConfig not implemented")
}

func (UnimplementedProviderServer) ValidateResourceConfig(context.Context, *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateResourceConfig not implemented")
}

func (UnimplementedProviderServer) ValidateDataResourceConfig(context.Context, *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateDataResourceConfig not implemented")
}

func (UnimplementedProviderServer) UpgradeResourceState(context.Context, *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpgradeResourceState not implemented")
}

func (UnimplementedProviderServer) ConfigureProvider(context.Context, *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ConfigureProvider not implemented")
}

func (UnimplementedProviderServer) ReadResource(context.Context, *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadResource not implemented")
}

func (UnimplementedProviderServer) PlanResourceChange(context.Context, *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PlanResourceChange not implemented")
}

func (UnimplementedProviderServer) ApplyResourceChange(context.Context, *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ApplyResourceChange not implemented")
}

func (UnimplementedProviderServer) ImportResourceState(context.Context, *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ImportResourceState not implemented")
}

func (UnimplementedProviderServer) MoveResourceState(context.Context, *tfprotov6.MoveResourceStateRequest) (*tfprotov6.MoveResourceStateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MoveResourceState not implemented")
}

func (UnimplementedProviderServer) ReadDataSource(context.Context, *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadDataSource not implemented")
}

func (UnimplementedProviderServer) GetFunctions(context.Context, *tfprotov6.GetFunctionsRequest) (*tfprotov6.GetFunctionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFunctions not implemented")
}

func (UnimplementedProviderServer) CallFunction(context.Context, *tfprotov6.CallFunctionRequest) (*tfprotov6.CallFunctionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CallFunction not implemented")
}

func (UnimplementedProviderServer) StopProvider(context.Context, *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopProvider not implemented")
}
