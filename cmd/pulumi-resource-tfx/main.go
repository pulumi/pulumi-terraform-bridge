package main

import (
	"context"
	"os"
	"os/signal"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/pulumi/pkg/v2/resource/provider"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"

	tfx "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/provider"
)

func main() {
	if err := provider.Main("tfx", func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		p, err := tfx.New(nil, host)
		if err != nil {
			return nil, err
		}

		signals := make(chan os.Signal)
		go func() {
			for _ = range signals {
				_, err := p.Cancel(context.Background(), &pbempty.Empty{})
				contract.IgnoreError(err)
				break
			}
		}()
		signal.Notify(signals, os.Interrupt, os.Kill)

		return p, nil
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
