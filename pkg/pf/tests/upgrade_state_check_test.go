// Copyright 2016-2025, Pulumi Corporation.
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

package tfbridgetests

import (
	"bytes"
	"context"
	"encoding/json"
	//"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	//pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	//"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	// crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	// "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
	// "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// Verify state upgrade interaction compatibility on schema change. This is a Plugin Framework port of similar test
// helpers for SDKv2 based providers.
//
// Play-by-play:
//
//  1. provision Inputs1 with the provider based on Resource1 schema
//  2. build the provider based on Resource2 schema
//  3. refresh with the Resource2 provider
//  4. update to Inputs2 with the Resource2 provider
type upgradeStateTestCase struct {
	Resource1     rschema.Resource
	ResourceInfo1 *info.Resource
	Inputs1       tftypes.Value
	InputsMap1    resource.PropertyMap
	Resource2     rschema.Resource
	ResourceInfo2 *info.Resource
	Inputs2       tftypes.Value
	InputsMap2    resource.PropertyMap

	ExpectFailure        bool // expect the test to fail, as in downgrades
	ExpectedRawStateType tftypes.Type

	SkipPulumi                        string // Reason to skip Pulumi side of the test
	SkipSchemaVersionAfterUpdateCheck bool
}

func (upgradeStateTestCase) tfProviderName() string {
	return "upgradeprovider"
}

type upgradeStateTestPhase string

const (
	createPhase  upgradeStateTestPhase = "create"
	refreshPhase upgradeStateTestPhase = "refresh"
	previewPhase upgradeStateTestPhase = "preview"
	updatePhase  upgradeStateTestPhase = "update"
)

// Represents an observed call to a state upgrade function.
type upgradeStateTrace struct {
	Phase    upgradeStateTestPhase // Phase in the test when the upgrader was called
	Upgrader int                   // StateUpgrader index in upgraders array that was called
	Request  rschema.UpgradeStateRequest
	Response rschema.UpgradeStateResponse
}

type upgradeStateResult struct {
	pulumiUpgrades      []upgradeStateTrace
	tfUpgrades          []upgradeStateTrace
	pulumiRefreshResult auto.RefreshResult
	pulumiPreviewResult auto.PreviewResult
	pulumiUpResult      auto.UpResult
}

func runUpgradeStateTest(t *testing.T, tc upgradeStateTestCase) upgradeStateResult {
	t.Helper()
	result := upgradeStateResult{}

	t.Run("tf", func(t *testing.T) {
		tcTF := instrumentModifyPlan(t, tc)
		tcTF = instrumentUpdate(t, tcTF)
		result.tfUpgrades = runUpgradeStateTestTF(t, tcTF)
	})

	t.Run("pulumi", func(t *testing.T) {
		if tc.SkipPulumi != "" {
			t.Skip(tc.SkipPulumi)
		}
		tcPulumi := instrumentModifyPlan(t, tc)
		tcPulumi = instrumentUpdate(t, tcPulumi)
		// resultPulumi := runUpgradeTestStatePulumi(t, tcPulumi)
		// result.pulumiUpgrades = resultPulumi.pulumiUpgrades
		// result.pulumiRefreshResult = resultPulumi.pulumiRefreshResult
		// result.pulumiPreviewResult = resultPulumi.pulumiPreviewResult
		// result.pulumiUpResult = resultPulumi.pulumiUpResult
	})

	return result
}

// Making sure the State.Raw received is of an appropriate type.
func checkRawState(t *testing.T, tc upgradeStateTestCase, receivedRawState tftypes.Value, method string) {
	if tc.ExpectedRawStateType == nil {
		return
	}
	receivedValueType := receivedRawState.Type()
	assert.Truef(t, tc.ExpectedRawStateType.Equal(receivedValueType),
		"%s expected State.Raw.Type() to be %s, got %s; the value is %v",
		method,
		tc.ExpectedRawStateType,
		receivedValueType.String(),
		receivedRawState.String())
}

func instrumentModifyPlan(t *testing.T, tc upgradeStateTestCase) upgradeStateTestCase {
	counter := new(atomic.Int32)

	r2, already := tc.Resource2.(rschema.ResourceWithModifyPlan)
	require.Falsef(t, already, "ResourceWithModifyPlan resources cannot yet be used in these tests")

	r2m := &resourceWithInstrumentedModifyPlan{
		Resource: r2,
		t:        t,
		tc:       tc,
		counter:  counter,
	}
	tc.Resource2 = r2m
	if !tc.ExpectFailure {
		t.Cleanup(func() {
			n := counter.Load()
			assert.Truef(t, n > 0, "expected ModifyPlan to be called at least once, got %d calls", n)
		})
	}
	return tc
}

type resourceWithInstrumentedModifyPlan struct {
	t  *testing.T
	tc upgradeStateTestCase
	rschema.Resource
	counter *atomic.Int32
}

func (r *resourceWithInstrumentedModifyPlan) ModifyPlan(
	ctx context.Context,
	req rschema.ModifyPlanRequest,
	resp *rschema.ModifyPlanResponse,
) {
	r.counter.Add(1)
	checkRawState(r.t, r.tc, req.Config.Raw, "ModifyPlan")
}

var _ rschema.ResourceWithModifyPlan = (*resourceWithInstrumentedModifyPlan)(nil)

func instrumentUpdate(t *testing.T, tc upgradeStateTestCase) upgradeStateTestCase {
	r2 := tc.Resource2
	tc.Resource2 = &resourceWithInstrumentedUpdate{
		Resource: r2,
		tc:       tc,
		t:        t,
	}
	return tc
}

type resourceWithInstrumentedUpdate struct {
	rschema.Resource
	t  *testing.T
	tc upgradeStateTestCase
}

func (r *resourceWithInstrumentedUpdate) Update(
	ctx context.Context,
	req rschema.UpdateRequest,
	resp *rschema.UpdateResponse,
) {
	checkRawState(r.t, r.tc, req.State.Raw, "Update")
	r.Resource.Update(ctx, req, resp)
}

type upgraderTracker struct {
	phase upgradeStateTestPhase
	trace []upgradeStateTrace
	mu    sync.Mutex
}

func (t *upgraderTracker) instrumentUpgrader(i int, u rschema.StateUpgrader) rschema.StateUpgrader {
	upgrade := u.StateUpgrader
	return rschema.StateUpgrader{
		PriorSchema: u.PriorSchema,
		StateUpgrader: func(
			ctx context.Context,
			req rschema.UpgradeStateRequest,
			resp *rschema.UpgradeStateResponse,
		) {
			t.mu.Lock()
			defer t.mu.Unlock()
			upgrade(ctx, req, resp)
			t.trace = append(t.trace, upgradeStateTrace{
				Phase:    t.phase,
				Upgrader: i,
				Request:  req,
				Response: *resp,
			})
		},
	}
}

type resourceWithInstrumentedUpgraders struct {
	rschema.Resource
	tr *upgraderTracker
}

func (r *resourceWithInstrumentedUpgraders) UpgradeState(ctx context.Context) map[int64]rschema.StateUpgrader {
	m := map[int64]rschema.StateUpgrader{}
	if old, ok := r.Resource.(rschema.ResourceWithUpgradeState); ok {
		m = old.UpgradeState(ctx)
	}
	for k := range m {
		m[k] = m[k]
	}
	return m
}

func instrumentUpgraders(s rschema.Resource) (rschema.Resource, *upgraderTracker) {
	tr := &upgraderTracker{}
	return &resourceWithInstrumentedUpgraders{Resource: s, tr: tr}, tr
}

var _ rschema.ResourceWithUpgradeState = (*resourceWithInstrumentedUpgraders)(nil)

func getVersionInState(t *testing.T, stack apitype.UntypedDeployment) int {
	data, err := stack.Deployment.MarshalJSON()
	require.NoError(t, err)

	var stateMap map[string]interface{}
	err = json.Unmarshal(data, &stateMap)
	require.NoError(t, err)

	resourcesList := stateMap["resources"].([]interface{})
	require.Len(t, resourcesList, 3)
	testResState := resourcesList[2].(map[string]interface{})
	resOutputs := testResState["outputs"].(map[string]interface{})
	metaVar := resOutputs[reservedkeys.Meta]
	if metaVar == nil {
		t.Logf("The resource does not have a meta field, assume the schema version is 0")
		return 0
	}
	meta := metaVar.(string)
	var metaMap map[string]interface{}
	err = json.Unmarshal([]byte(meta), &metaMap)
	require.NoError(t, err)
	schemaVersion, err := strconv.ParseInt(metaMap["schema_version"].(string), 10, 64)
	require.NoError(t, err)
	return int(schemaVersion)
}

// func upgradeTestBrigedProvider(t *testing.T, r *schema.Resource, ri *info.Resource) info.Provider {
// 	tfp := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: r}}
// 	p := pulcheck.BridgedProvider(t, defProviderShortName, tfp)
// 	if ri != nil {
// 		resourceInfo := *ri
// 		resourceInfo.Tok = p.Resources[defRtype].Tok
// 		p.Resources[defRtype] = &resourceInfo
// 	}
// 	return p
// }

func getSchemaVersion(res rschema.Resource) int64 {
	resp := &rschema.SchemaResponse{}
	res.Schema(context.Background(), rschema.SchemaRequest{}, resp)
	contract.Assertf(!resp.Diagnostics.HasError(), "res.Schema() returned error diagnostics: %v", resp.Diagnostics)
	return resp.Schema.Version
}

// func runUpgradeTestStatePulumi(t *testing.T, tc upgradeStateTestCase) upgradeStateResult {

// 	res1 := tc.Resource1
// 	res2, tracker := instrumentUpgraders(tc.Resource2)

// 	prov1 := upgradeTestBrigedProvider(t, res1, tc.ResourceInfo1)
// 	prov2 := upgradeTestBrigedProvider(t, res2, tc.ResourceInfo2)

// 	pd := &pulumiDriver{
// 		name:                defProviderShortName,
// 		pulumiResourceToken: defRtoken,
// 		tfResourceName:      defRtype,
// 	}

// 	inputs1 := coalesceInputs(t, tc.Resource1.Schema, tc.Inputs1)
// 	inputs2 := coalesceInputs(t, tc.Resource2.Schema, tc.Inputs2)

// 	pm1 := tc.InputsMap1
// 	if pm1 == nil {
// 		sch := prov1.P.ResourcesMap().Get(pd.tfResourceName).Schema()
// 		info := tc.ResourceInfo1.GetFields()
// 		pm1 = crosstestsimpl.InferPulumiValue(t, sch, info, inputs1)
// 	}

// 	yamlProgram := pd.generateYAML(t, pm1)
// 	pt := pulcheck.PulCheck(t, prov1, string(yamlProgram))

// 	t.Logf("#### create")
// 	tracker.phase = createPhase
// 	createResult := pt.Up(t)
// 	t.Logf("%s", createResult.StdOut+createResult.StdErr)

// 	createdState := pt.ExportStack(t)

// 	schemaVersion1 := getVersionInState(t, createdState)
// 	require.Equalf(t, getSchemaVersion(tc.Resource1), schemaVersion1, "bad getVersionInState result for create")

// 	pm2 := tc.InputsMap2
// 	if pm2 == nil {
// 		sch := prov1.P.ResourcesMap().Get(pd.tfResourceName).Schema()
// 		info := tc.ResourceInfo2.GetFields()
// 		pm2 = crosstestsimpl.InferPulumiValue(t, sch, info, inputs2)
// 	}

// 	yamlProgram = pd.generateYAML(t, pm2)
// 	p := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
// 	err := os.WriteFile(p, yamlProgram, 0o600)
// 	require.NoErrorf(t, err, "writing Pulumi.yaml")

// 	handle, err := pulcheck.StartPulumiProvider(context.Background(), prov2)
// 	require.NoError(t, err)
// 	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS",
// 		fmt.Sprintf("%s:%d", defProviderShortName, handle.Port))

// 	t.Logf("#### refresh")
// 	tracker.phase = refreshPhase
// 	refreshResult := pt.Refresh(t)
// 	t.Logf("%s", refreshResult.StdOut+refreshResult.StdErr)

// 	schemaVersionR := getVersionInState(t, pt.ExportStack(t))
// 	t.Logf("schema version after refresh is %d", schemaVersionR)
// 	require.Equalf(t, getSchemaVersion(tc.Resource2), schemaVersionR, "bad getVersionInState result for refresh")

// 	// Reset to created state as refresh may have edited it.
// 	pt.ImportStack(t, createdState)

// 	t.Logf("#### preview")
// 	tracker.phase = previewPhase
// 	previewResult := pt.Preview(t, optpreview.Diff())
// 	t.Logf("%s", previewResult.StdOut+previewResult.StdErr)

// 	t.Logf("#### update")
// 	tracker.phase = updatePhase
// 	updateResult := pt.Up(t) // --skip-preview would be nice here
// 	t.Logf("%s", updateResult.StdOut+updateResult.StdErr)

// 	schemaVersionU := getVersionInState(t, pt.ExportStack(t))
// 	t.Logf("schema version after update is %d", schemaVersionU)
// 	if !tc.SkipSchemaVersionAfterUpdateCheck {
// 		require.Equalf(t, getSchemaVersion(tc.Resource2), schemaVersionU,
// 			"bad getVersionInState result for update")
// 	}

// 	return upgradeStateResult{
// 		pulumiUpgrades:      tracker.trace,
// 		pulumiPreviewResult: previewResult,
// 		pulumiRefreshResult: refreshResult,
// 		pulumiUpResult:      updateResult,
// 	}
// }

func (tc upgradeStateTestCase) tfProviderServerBuilder(resource rschema.Resource) interface {
	GRPCProvider() tfprotov6.ProviderServer
} {
	return &upgradeStateTFProvider{
		resource: resource,
		name:     tc.tfProviderName(),
		version:  "0.0.1",
	}
}

type upgradeStateTFProvider struct {
	resource rschema.Resource
	name     string
	version  string
}

func (p *upgradeStateTFProvider) Metadata(
	_ context.Context,
	_ provider.MetadataRequest,
	resp *provider.MetadataResponse,
) {
	resp.TypeName = p.name
	resp.Version = p.version
}

func (p *upgradeStateTFProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {
}

func (p *upgradeStateTFProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (p *upgradeStateTFProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (p *upgradeStateTFProvider) Resources(context.Context) []func() rschema.Resource {
	return []func() rschema.Resource{
		func() rschema.Resource {
			return p.resource
		},
	}
}

var _ provider.Provider = &upgradeStateTFProvider{}

func (*upgradeStateTFProvider) GRPCProvider() tfprotov6.ProviderServer {
	mkProvider := providerserver.NewProtocol6(nil)
	return mkProvider()
}

func runUpgradeStateTestTF(t *testing.T, tc upgradeStateTestCase) []upgradeStateTrace {
	t.Logf("Checking TF behavior")

	tfwd := t.TempDir()

	resource1 := tc.Resource1
	resource2, tracker := instrumentUpgraders(tc.Resource2)

	tfd1 := tfcheck.NewTfDriver(t, tfwd, tc.tfProviderName(), tc.tfProviderServerBuilder(tc.Resource1))

	t.Logf("#### create")
	upgradeStateWriteHCL(t, tc, tfwd, resource1, tc.Inputs1)
	tracker.phase = createPhase

	// TODO write HCL1

	plan, err := tfd1.Plan(t)
	require.NoErrorf(t, err, "tfd1.Plan failed")
	err = tfd1.Apply(t, plan)
	require.NoErrorf(t, err, "tfd1.Apply failed")

	tfd2 := tfcheck.NewTfDriver(t, tfwd, tc.tfProviderName(), tc.tfProviderServerBuilder(resource2))

	t.Logf("#### save current state as created state")
	stateFile := filepath.Join(tfwd, "terraform.tfstate")
	createdState, err := os.ReadFile(stateFile)
	require.NoErrorf(t, err, "saving state failed")

	t.Logf("#### refresh")
	upgradeStateWriteHCL(t, tc, tfwd, resource2, tc.Inputs2)
	tracker.phase = refreshPhase
	err = tfd2.Refresh(t)
	if tc.ExpectFailure {
		require.Errorf(t, err, "refresh should have failed")
	} else {
		require.NoErrorf(t, err, "refresh should not have failed")
	}

	// Reset the state to the created state check apply, as refresh has migrated the state.
	t.Logf("#### reset state to created state")
	err = os.WriteFile(stateFile, createdState, 0o600)
	require.NoErrorf(t, err, "resetting state failed")

	t.Logf("#### plan (similar to preview)")
	tracker.phase = previewPhase
	plan, err = tfd2.Plan(t)
	if tc.ExpectFailure {
		require.Errorf(t, err, "refresh should have failed")
		return tracker.trace
	}
	require.NoErrorf(t, err, "refresh should not have failed")

	t.Logf("#### apply (similar to update)")
	tracker.phase = updatePhase
	err = tfd2.Apply(t, plan)
	require.NoErrorf(t, err, "tfd2.Apply failed")

	return tracker.trace
}

func upgradeStateWriteHCL(t *testing.T, tc upgradeStateTestCase, pwd string, res rschema.Resource, v tftypes.Value) {
	var buf bytes.Buffer
	tn := getResourceTypeName(t, tc.tfProviderName(), res)
	hclWriteResource(t, &buf, tn, res, "example", v)
	t.Logf("HCL: %s", buf.String())
	err := os.WriteFile(filepath.Join(pwd, "infra.tf"), buf.Bytes(), 0o700)
	require.NoErrorf(t, err, "Failed to write infra.tf")
}

func nopUpgrade(
	ctx context.Context,
	rawState map[string]interface{},
	meta interface{},
) (map[string]interface{}, error) {
	return rawState, nil
}

func skipUnlessLinux(t *testing.T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}

func getResourceTypeName(t *testing.T, providerTypeName string, res rschema.Resource) string {
	resp := &rschema.MetadataResponse{}
	res.Metadata(context.Background(), rschema.MetadataRequest{
		ProviderTypeName: providerTypeName,
	}, resp)
	return resp.TypeName
}
