package tfplugin5

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/tfplugin5/proto"
)

var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  5,
	MagicCookieKey:   "TF_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2",
}

type providerPlugin struct {
	plugin.Plugin

	terraformVersion string
}

func (p *providerPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker,
	c *grpc.ClientConn) (interface{}, error) {

	return NewProvider(ctx, proto.NewProviderClient(c), p.terraformVersion)
}

func (p *providerPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return fmt.Errorf("unsupported")
}

func StartProvider(ctx context.Context, executablePath, terraformVersion string) (shim.Provider, error) {
	var logger hclog.Logger
	switch os.Getenv("TF_LOG") {
	case "TRACE":
		logger = hclog.New(&hclog.LoggerOptions{Level: hclog.Trace})
	case "DEBUG":
		logger = hclog.New(&hclog.LoggerOptions{Level: hclog.Debug})
	case "INFO":
		logger = hclog.New(&hclog.LoggerOptions{Level: hclog.Info})
	case "WARN":
		logger = hclog.New(&hclog.LoggerOptions{Level: hclog.Warn})
	case "ERROR":
		logger = hclog.New(&hclog.LoggerOptions{Level: hclog.Error})
	default:
		logger = hclog.NewNullLogger()
	}

	pluginClient := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          plugin.PluginSet{"provider": &providerPlugin{terraformVersion: terraformVersion}},
		Cmd:              exec.Command(executablePath),
		Managed:          true,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		AutoMTLS:         true,
		Logger:           logger,
	})
	go func() {
		<-ctx.Done()
		pluginClient.Kill()
	}()

	client, err := pluginClient.Client()
	if err != nil {
		return nil, err
	}
	provider, err := client.Dispense("provider")
	if err != nil {
		return nil, err
	}
	return provider.(shim.Provider), nil
}
