package pulcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/go/common/util/contract"
	"github.com/stretchr/testify/require"
)

type tfDriver struct {
	cwd            string
	providerName   string
	reattachConfig *plugin.ReattachConfig
}

type tfPlan struct {
	PlanFile string
	RawPlan  any
}

func getTFCommand() string {
	if cmd := os.Getenv("TF_COMMAND_OVERRIDE"); cmd != "" {
		return cmd
	}
	return "terraform"
}

func newTfDriver(t T, dir, providerName string, prov *schema.Provider) *tfDriver {
	// Did not find a less intrusive way to disable annoying logging:
	os.Setenv("TF_LOG_PROVIDER", "off")
	os.Setenv("TF_LOG_SDK", "off")
	os.Setenv("TF_LOG_SDK_PROTO", "off")

	EnsureProviderValid(t, prov)

	serverFactory := func() tfprotov5.ProviderServer {
		return prov.GRPCProvider()
	}

	ctx := context.Background()

	reattachConfigCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})

	serveOpts := []tf5server.ServeOpt{
		tf5server.WithGoPluginLogger(hclog.FromStandardLogger(log.New(io.Discard, "", 0), hclog.DefaultOptions)),
		tf5server.WithDebug(ctx, reattachConfigCh, closeCh),
		tf5server.WithoutLogStderrOverride(),
	}

	go func() {
		err := tf5server.Serve(providerName, serverFactory, serveOpts...)
		require.NoError(t, err)
	}()

	reattachConfig := <-reattachConfigCh
	return &tfDriver{
		providerName:   providerName,
		cwd:            dir,
		reattachConfig: reattachConfig,
	}
}

func (d *tfDriver) write(t T, program string) {
	t.Logf("HCL: \n%s\n", program)
	err := os.WriteFile(filepath.Join(d.cwd, "test.tf"), []byte(program), 0o600)
	require.NoErrorf(t, err, "writing test.tf")
}

func (d *tfDriver) plan(t T) *tfPlan {
	planFile := filepath.Join(d.cwd, "test.tfplan")
	env := []string{d.formatReattachEnvVar()}
	tfCmd := getTFCommand()
	execCmd(t, d.cwd, env, tfCmd, "plan", "-refresh=false", "-out", planFile)
	cmd := execCmd(t, d.cwd, env, tfCmd, "show", "-json", planFile)
	tp := tfPlan{PlanFile: planFile}
	err := json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &tp.RawPlan)
	require.NoErrorf(t, err, "failed to unmarshal terraform plan")
	return &tp
}

func (d *tfDriver) apply(t T, plan *tfPlan) {
	tfCmd := getTFCommand()
	execCmd(t, d.cwd, []string{d.formatReattachEnvVar()},
		tfCmd, "apply", "-auto-approve", "-refresh=false", plan.PlanFile)
}

func (d *tfDriver) show(t T, planFile string) string {
	tfCmd := getTFCommand()
	cmd := execCmd(t, d.cwd, []string{d.formatReattachEnvVar()}, tfCmd, "show", "-json", planFile)
	return cmd.Stdout.(*bytes.Buffer).String()
}

func (d *tfDriver) formatReattachEnvVar() string {
	name := d.providerName
	pluginReattachConfig := d.reattachConfig

	type reattachConfigAddr struct {
		Network string
		String  string
	}

	type reattachConfig struct {
		Protocol        string
		ProtocolVersion int
		Pid             int
		Test            bool
		Addr            reattachConfigAddr
	}

	reattachBytes, err := json.Marshal(map[string]reattachConfig{
		name: {
			Protocol:        string(pluginReattachConfig.Protocol),
			ProtocolVersion: pluginReattachConfig.ProtocolVersion,
			Pid:             pluginReattachConfig.Pid,
			Test:            pluginReattachConfig.Test,
			Addr: reattachConfigAddr{
				Network: pluginReattachConfig.Addr.Network(),
				String:  pluginReattachConfig.Addr.String(),
			},
		},
	})

	contract.AssertNoErrorf(err, "failed to build TF_REATTACH_PROVIDERS string")
	return fmt.Sprintf("TF_REATTACH_PROVIDERS=%s", string(reattachBytes))
}
