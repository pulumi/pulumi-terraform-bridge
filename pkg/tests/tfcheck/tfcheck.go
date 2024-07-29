package tfcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi/sdk/go/common/util/contract"
	"github.com/stretchr/testify/require"
)

type TfDriver struct {
	cwd            string
	providerName   string
	reattachConfig *plugin.ReattachConfig
}

type TfPlan struct {
	PlanFile string
	RawPlan  any
}

func getTFCommand() string {
	if cmd := os.Getenv("TF_COMMAND_OVERRIDE"); cmd != "" {
		return cmd
	}
	return "terraform"
}

func skipUnlessLinux(t pulcheck.T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}

func disableTFLogging() {
	// Did not find a less intrusive way to disable annoying logging:
	os.Setenv("TF_LOG_PROVIDER", "off")
	os.Setenv("TF_LOG_SDK", "off")
	os.Setenv("TF_LOG_SDK_PROTO", "off")
}

type providerv5 interface {
	GRPCProvider() tfprotov5.ProviderServer
}

type providerv6 interface {
	GRPCProvider() tfprotov6.ProviderServer
}

func NewTfDriverSDK(t pulcheck.T, dir, providerName string, prov *schema.Provider) *TfDriver {
	pulcheck.EnsureProviderValid(t, prov)
	return newTFDriverV5(t, dir, providerName, prov)
}

func newTFDriverV5(t pulcheck.T, dir, providerName string, prov providerv5) *TfDriver {
	skipUnlessLinux(t)
	disableTFLogging()

	ctx := context.Background()

	reattachConfigCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})

	serverFactory := func() tfprotov5.ProviderServer {
		return prov.GRPCProvider()
	}

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
	return &TfDriver{
		providerName:   providerName,
		cwd:            dir,
		reattachConfig: reattachConfig,
	}
}

// Only exported for use in PF
func NewTFDriverV6(t pulcheck.T, dir, providerName string, prov providerv6) *TfDriver {
	skipUnlessLinux(t)
	disableTFLogging()

	ctx := context.Background()

	reattachConfigCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})

	serverFactory := func() tfprotov6.ProviderServer {
		return prov.GRPCProvider()
	}

	serverOpts := []tf6server.ServeOpt{
		tf6server.WithGoPluginLogger(hclog.FromStandardLogger(log.New(io.Discard, "", 0), hclog.DefaultOptions)),
		tf6server.WithDebug(ctx, reattachConfigCh, closeCh),
		tf6server.WithoutLogStderrOverride(),
	}

	go func() {
		err := tf6server.Serve(providerName, serverFactory, serverOpts...)
		require.NoError(t, err)
	}()

	reattachConfig := <-reattachConfigCh
	return &TfDriver{
		providerName:   providerName,
		cwd:            dir,
		reattachConfig: reattachConfig,
	}
}

func (d *TfDriver) Write(t pulcheck.T, program string) {
	t.Logf("HCL: \n%s\n", program)
	err := os.WriteFile(filepath.Join(d.cwd, "test.tf"), []byte(program), 0o600)
	require.NoErrorf(t, err, "writing test.tf")
}

func (d *TfDriver) Plan(t pulcheck.T) *TfPlan {
	planFile := filepath.Join(d.cwd, "test.tfplan")
	env := []string{d.formatReattachEnvVar()}
	tfCmd := getTFCommand()
	execCmd(t, d.cwd, env, tfCmd, "plan", "-refresh=false", "-out", planFile)
	cmd := execCmd(t, d.cwd, env, tfCmd, "show", "-json", planFile)
	tp := TfPlan{PlanFile: planFile}
	err := json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &tp.RawPlan)
	require.NoErrorf(t, err, "failed to unmarshal terraform plan")
	return &tp
}

func (d *TfDriver) Apply(t pulcheck.T, plan *TfPlan) {
	tfCmd := getTFCommand()
	execCmd(t, d.cwd, []string{d.formatReattachEnvVar()},
		tfCmd, "apply", "-auto-approve", "-refresh=false", plan.PlanFile)
}

func (d *TfDriver) Show(t pulcheck.T, planFile string) string {
	tfCmd := getTFCommand()
	cmd := execCmd(t, d.cwd, []string{d.formatReattachEnvVar()}, tfCmd, "show", "-json", planFile)
	res := cmd.Stdout.(*bytes.Buffer)
	buf := bytes.NewBuffer(nil)
	err := json.Indent(buf, res.Bytes(), "", "    ")
	require.NoError(t, err)
	return buf.String()
}

func (d *TfDriver) GetState(t pulcheck.T) string {
	res, err := os.ReadFile(path.Join(d.cwd, "terraform.tfstate"))
	require.NoError(t, err)
	buf := bytes.NewBuffer(nil)
	err = json.Indent(buf, res, "", "    ")
	require.NoError(t, err)
	return buf.String()
}

func (d *TfDriver) GetOutput(t pulcheck.T, outputName string) string {
	tfCmd := getTFCommand()
	cmd := execCmd(t, d.cwd, []string{d.formatReattachEnvVar()}, tfCmd, "output", outputName)
	res := cmd.Stdout.(*bytes.Buffer).String()
	res = strings.TrimSuffix(res, "\n")
	res = strings.Trim(res, "\"")
	return res
}

func (d *TfDriver) formatReattachEnvVar() string {
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

