package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"testing"
	"time"

	"fmt"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

type tfProxyProviderServer struct {
	UnimplementedProviderServer
}

func (*tfProxyProviderServer) GetMetadata(ctx context.Context, req *tfprotov6.GetMetadataRequest) (*tfprotov6.GetMetadataResponse, error) {
	return &tfprotov6.GetMetadataResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "GetMetadata Summary",
				Detail:   "GetMetadata Detail",
			},
		},
	}, nil
}

func (*tfProxyProviderServer) GetProviderSchema(ctx context.Context, req *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	return &tfprotov6.GetProviderSchemaResponse{Diagnostics: []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "GetProviderSchema Summary",
			Detail:   "GetProviderSchema Detail",
		},
	}}, nil
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

func startTFProviderProxy(providerName string) (*tfProviderProxyHandle, error) {
	serverHandle, reattachConfig, err := simpleServe(providerName, func() tfprotov6.ProviderServer {
		return &tfProxyProviderServer{}
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
		managedDebugReattachConfigTimeout: 2 * time.Second,
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
