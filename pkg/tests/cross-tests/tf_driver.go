package crosstests

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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
)

type tfDriver struct {
	cwd            string
	providerName   string
	reattachConfig *plugin.ReattachConfig
	res            *schema.Resource
}

type tfPlan struct {
	PlanFile string
	RawPlan  any
}

func newTfDriver(t T, dir, providerName, resName string, res *schema.Resource) *tfDriver {
	// Did not find a less intrusive way to disable annoying logging:
	os.Setenv("TF_LOG_PROVIDER", "off")
	os.Setenv("TF_LOG_SDK", "off")
	os.Setenv("TF_LOG_SDK_PROTO", "off")

	// res.CustomizeDiff = func(
	// 	ctx context.Context, rd *schema.ResourceDiff, i interface{},
	// ) error {
	// 	return nil
	// }

	if res.DeleteContext == nil {
		res.DeleteContext = func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			return diag.Diagnostics{}
		}
	}

	if res.CreateContext == nil {
		res.CreateContext = func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			rd.SetId("newid")
			return diag.Diagnostics{}
		}
	}

	res.UpdateContext = func(
		ctx context.Context, rd *schema.ResourceData, i interface{},
	) diag.Diagnostics {
		return diag.Diagnostics{}
	}

	p := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			resName: res,
		},
	}

	serverFactory := func() tfprotov5.ProviderServer {
		return p.GRPCProvider()
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
		res:            res,
	}
}

func (d *tfDriver) coalesce(t T, x any) *tftypes.Value {
	if x == nil {
		return nil
	}
	objectType := convert.InferObjectType(sdkv2.NewSchemaMap(d.res.Schema), nil)
	t.Logf("infer object type: %v", objectType)
	v := FromType(objectType).NewValue(x)
	return &v
}

func (d *tfDriver) writePlanApply(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	rawConfig any,
) *tfPlan {
	config := d.coalesce(t, rawConfig)
	if config != nil {
		d.write(t, resourceSchema, resourceType, resourceName, *config)
	}
	plan := d.plan(t)
	d.apply(t, plan)
	return plan
}

func (d *tfDriver) write(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	config tftypes.Value,
) {
	var buf bytes.Buffer
	err := WriteHCL(&buf, resourceSchema, resourceType, resourceName, FromValue(config).ToCty())
	t.Logf("HCL: \n%s\n", buf.String())
	bytes := buf.Bytes()
	err = os.WriteFile(filepath.Join(d.cwd, "test.tf"), bytes, 0600)
	require.NoErrorf(t, err, "writing test.tf")
}

func (d *tfDriver) plan(t T) *tfPlan {
	planFile := filepath.Join(d.cwd, "test.tfplan")
	env := []string{d.formatReattachEnvVar()}
	execCmd(t, d.cwd, env, "terraform", "plan", "-refresh=false", "-out", planFile)
	cmd := execCmd(t, d.cwd, env, "terraform", "show", "-json", planFile)
	tp := tfPlan{PlanFile: planFile}
	err := json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &tp.RawPlan)
	require.NoErrorf(t, err, "failed to unmarshal terraform plan")
	return &tp
}

func (d *tfDriver) apply(t T, plan *tfPlan) {
	execCmd(t, d.cwd, []string{d.formatReattachEnvVar()},
		"terraform", "apply", "-auto-approve", "-refresh=false", plan.PlanFile)
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
