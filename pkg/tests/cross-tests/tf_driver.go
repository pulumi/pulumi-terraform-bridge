// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Helper code to drive Terraform CLI to run tests against an in-process provider.
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
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/internal/pulcheck"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
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

	p := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			resName: res,
		},
	}
	pulcheck.EnsureProviderValid(t, p)

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
	for k := range objectType.AttributeTypes {
		objectType.OptionalAttributes[k] = struct{}{}
	}
	t.Logf("infer object type: %v", objectType)
	v := fromType(objectType).NewValue(x)
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
	err := WriteHCL(&buf, resourceSchema, resourceType, resourceName, fromValue(config).ToCty())
	require.NoError(t, err)
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

// Still discovering the structure of JSON-serialized TF plans. The information required from these is, primarily, is
// whether the resource is staying unchanged, being updated or replaced. Secondarily, would be also great to know
// detailed paths of properties causing the change, though that is more difficult to cross-compare with Pulumi.
//
// For now this is code is similar to `jq .resource_changes[0].change.actions[0] plan.json`.
func (*tfDriver) parseChangesFromTFPlan(plan tfPlan) string {
	type p struct {
		ResourceChanges []struct {
			Change struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	jb, err := json.Marshal(plan.RawPlan)
	contract.AssertNoErrorf(err, "failed to marshal terraform plan")
	var pp p
	err = json.Unmarshal(jb, &pp)
	contract.AssertNoErrorf(err, "failed to unmarshal terraform plan")
	contract.Assertf(len(pp.ResourceChanges) == 1, "expected exactly one resource change")
	actions := pp.ResourceChanges[0].Change.Actions
	contract.Assertf(len(actions) == 1, "expected exactly one action, got %v", strings.Join(actions, ", "))
	return actions[0]
}
