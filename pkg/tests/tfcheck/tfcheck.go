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
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

type TFDriver struct {
	cwd            string
	providerName   string
	reattachConfig *plugin.ReattachConfig
}

type TFPlan struct {
	StdOut   string
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

type providerv6 interface {
	GRPCProvider() tfprotov6.ProviderServer
}

// This takes a sdkv2 schema.Provider or a providerv6
func NewTfDriver(t pulcheck.T, dir, providerName string, prov any) *TFDriver {
	switch p := prov.(type) {
	case *schema.Provider:
		return newTfDriverSDK(t, dir, providerName, p)
	case providerv6:
		return newTFDriverV6(t, dir, providerName, p.GRPCProvider())
	default:
		contract.Failf("unsupported provider type %T", prov)
		return nil
	}
}

func newTfDriverSDK(t pulcheck.T, dir, providerName string, prov *schema.Provider) *TFDriver {
	pulcheck.EnsureProviderValid(t, prov)
	v6server, err := tf5to6server.UpgradeServer(context.Background(),
		func() tfprotov5.ProviderServer { return prov.GRPCProvider() })
	require.NoError(t, err)
	return newTFDriverV6(t, dir, providerName, v6server)
}

func newTFDriverV6(t pulcheck.T, dir, providerName string, prov tfprotov6.ProviderServer) *TFDriver {
	skipUnlessLinux(t)
	disableTFLogging()

	ctx := context.Background()

	reattachConfigCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})

	serverFactory := func() tfprotov6.ProviderServer {
		return prov
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
	return &TFDriver{
		providerName:   providerName,
		cwd:            dir,
		reattachConfig: reattachConfig,
	}
}

func (d *TFDriver) Write(t pulcheck.T, program string) {
	t.Logf("HCL: \n%s\n", program)
	err := os.WriteFile(filepath.Join(d.cwd, "test.tf"), []byte(program), 0o600)
	require.NoErrorf(t, err, "writing test.tf")
}

func (d *TFDriver) Plan(t pulcheck.T) (*TFPlan, error) {
	planFile := filepath.Join(d.cwd, "test.tfplan")
	planStdoutBytes, err := d.execTf(t, "plan", "-refresh=false", "-out", planFile, "-no-color")
	if err != nil {
		return nil, err
	}
	planStdout := strings.Split(string(planStdoutBytes), "───")[0] // trim unstable output about the plan file
	stdout, err := d.execTf(t, "show", "-json", planFile)
	require.NoError(t, err)
	tp := TFPlan{PlanFile: planFile, StdOut: planStdout}
	err = json.Unmarshal(stdout, &tp.RawPlan)
	require.NoErrorf(t, err, "failed to unmarshal terraform plan")
	return &tp, nil
}

func (d *TFDriver) Apply(t pulcheck.T, plan *TFPlan) error {
	_, err := d.execTf(t, "apply", "-auto-approve", "-refresh=false", plan.PlanFile)
	return err
}

func (d *TFDriver) Show(t pulcheck.T, planFile string) string {
	res, err := d.execTf(t, "show", "-json", planFile)
	require.NoError(t, err)
	var dst bytes.Buffer
	err = json.Indent(&dst, res, "", "    ")
	require.NoError(t, err)
	return dst.String()
}

func (d *TFDriver) GetState(t pulcheck.T) string {
	res, err := os.ReadFile(path.Join(d.cwd, "terraform.tfstate"))
	require.NoError(t, err)
	buf := bytes.NewBuffer(nil)
	err = json.Indent(buf, res, "", "    ")
	require.NoError(t, err)
	return buf.String()
}

func (d *TFDriver) GetOutput(t pulcheck.T, outputName string) string {
	resB, err := d.execTf(t, "output", outputName)
	require.NoError(t, err)
	res := strings.TrimSuffix(string(resB), "\n")
	res = strings.Trim(res, "\"")
	return res
}

func (d *TFDriver) formatReattachEnvVar() string {
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

type TFChange struct {
	Actions []string       `json:"actions"`
	Before  map[string]any `json:"before"`
	After   map[string]any `json:"after"`
}

// Still discovering the structure of JSON-serialized TF plans. The information required from these is, primarily, is
// whether the resource is staying unchanged, being updated or replaced. Secondarily, would be also great to know
// detailed paths of properties causing the change, though that is more difficult to cross-compare with Pulumi.
//
// For now this is code is similar to `jq .resource_changes[0].change.actions[0] plan.json`.
func (*TFDriver) ParseChangesFromTFPlan(plan *TFPlan) TFChange {
	type p struct {
		ResourceChanges []struct {
			Change TFChange `json:"change"`
		} `json:"resource_changes"`
	}
	jb, err := json.Marshal(plan.RawPlan)
	contract.AssertNoErrorf(err, "failed to marshal terraform plan")
	var pp p
	err = json.Unmarshal(jb, &pp)
	contract.AssertNoErrorf(err, "failed to unmarshal terraform plan")
	contract.Assertf(len(pp.ResourceChanges) == 1, "expected exactly one resource change")
	return pp.ResourceChanges[0].Change
}
