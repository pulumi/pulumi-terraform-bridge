package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"google.golang.org/grpc"
)

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
		"Pid":             os.Getegid(),
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
	serverHandle, reattachConfig, err := simpleServe(providerName, nil, tf6server.WithManagedDebug())
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
	opts ...tf6server.ServeOpt,
) (io.Closer, *plugin.ReattachConfig, error) {
	// Defaults
	conf := simpleServeConfig{
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

	if conf.managedDebug {
		ctx, cancel := context.WithCancel(context.Background())
		signalCh := make(chan os.Signal, len(conf.managedDebugStopSignals))

		signal.Notify(signalCh, conf.managedDebugStopSignals...)

		defer func() {
			signal.Stop(signalCh)
			cancel()
		}()

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

	return &handleCloser{conf.debugCloseCh}, pluginReattachConfig, nil
}

// Helper for [simpleServe].
type handleCloser struct {
	debugCloseCh chan struct{}
}

func (hc *handleCloser) Close() error {
	// Wait for the server to be done.
	<-hc.debugCloseCh

	return nil
}
