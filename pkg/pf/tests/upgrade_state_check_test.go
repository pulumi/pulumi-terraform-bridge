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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
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
	Resource1     *pb.Resource
	ResourceInfo1 *info.Resource
	Inputs1       cty.Value
	InputsMap1    resource.PropertyMap
	Resource2     *pb.Resource
	ResourceInfo2 *info.Resource
	Inputs2       cty.Value
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
	Phase         upgradeStateTestPhase // Phase in the test when the upgrader was called
	Upgrader      int64                 // StateUpgrader identified by target version
	PriorState    any
	ReturnedState any
	ReturnedError bool
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
		resultPulumi := runUpgradeTestStatePulumi(t, tcPulumi)
		result.pulumiUpgrades = resultPulumi.pulumiUpgrades
		result.pulumiRefreshResult = resultPulumi.pulumiRefreshResult
		result.pulumiPreviewResult = resultPulumi.pulumiPreviewResult
		result.pulumiUpResult = resultPulumi.pulumiUpResult
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
	require.Nilf(t, tc.Resource2.ModifyPlanFunc, "ModifyPlanFunc resources cannot yet be used in these tests")

	r2m := *tc.Resource2

	r2m.ModifyPlanFunc = func(
		ctx context.Context,
		req rschema.ModifyPlanRequest,
		resp *rschema.ModifyPlanResponse,
	) {
		counter.Add(1)
		checkRawState(t, tc, req.Config.Raw, "ModifyPlan")
	}
	tc.Resource2 = &r2m
	if !tc.ExpectFailure {
		t.Cleanup(func() {
			n := counter.Load()
			assert.Truef(t, n > 0, "expected ModifyPlan to be called at least once, got %d calls", n)
		})
	}
	return tc
}

func instrumentUpdate(t *testing.T, tc upgradeStateTestCase) upgradeStateTestCase {
	r2m := *tc.Resource2
	upd := r2m.UpdateFunc
	r2m.UpdateFunc = func(ctx context.Context, req rschema.UpdateRequest, resp *rschema.UpdateResponse) {
		checkRawState(t, tc, req.State.Raw, "Update")
		if upd != nil {
			upd(ctx, req, resp)
		}
	}
	tc.Resource2 = &r2m
	return tc
}

type upgraderTracker struct {
	phase upgradeStateTestPhase
	trace []upgradeStateTrace
	mu    sync.Mutex
}

func (t *upgraderTracker) instrumentUpgrader(ver int64, u rschema.StateUpgrader) rschema.StateUpgrader {
	prettyPrintValue := func(v tftypes.Value) (any, error) {
		cv, err := convertTValueToCtyValue(v)
		if err != nil {
			return nil, err
		}
		bytes, err := ctyjson.Marshal(cv, cv.Type())
		if err != nil {
			return nil, err
		}

		var out any
		err = json.Unmarshal(bytes, &out)
		if err != nil {
			return nil, err
		}
		return out, nil
	}

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

			priorState, err := prettyPrintValue(req.State.Raw)
			if err != nil {
				resp.Diagnostics.AddError("prettyPrintValue failed on req", err.Error())
				return
			}

			newState, err := prettyPrintValue(resp.State.Raw)
			if err != nil {
				resp.Diagnostics.AddError("prettyPrintValue failed on resp", err.Error())
				return
			}

			t.trace = append(t.trace, upgradeStateTrace{
				Phase:         t.phase,
				Upgrader:      ver,
				PriorState:    priorState,
				ReturnedState: newState,
				ReturnedError: resp.Diagnostics.HasError(),
			})
		},
	}
}

func instrumentUpgraders(r *pb.Resource) (*pb.Resource, *upgraderTracker) {
	tr := &upgraderTracker{}
	rm := *r
	usf := r.UpgradeStateFunc
	rm.UpgradeStateFunc = func(ctx context.Context) map[int64]rschema.StateUpgrader {
		m := map[int64]rschema.StateUpgrader{}
		if usf != nil {
			m = usf(ctx)
		}
		for k, u := range m {
			m[k] = tr.instrumentUpgrader(k, u)
		}
		return m
	}
	return &rm, tr
}

func getVersionInState(t *testing.T, stack apitype.UntypedDeployment) int64 {
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
	return schemaVersion
}

func upgradeTestBrigedProvider(
	t *testing.T,
	tc upgradeStateTestCase,
	r rschema.Resource,
	ri *info.Resource,
) info.Provider {
	tn := getResourceTypeName(tc.tfProviderName(), r)
	return newPulumiTestProviderInfo(t, tc.tfProviderName(), tc.tfProvider(r), func(p *info.Provider) {
		if ri != nil {
			resourceInfo := *ri
			resourceInfo.Tok = p.Resources[tn].Tok
			p.Resources[tn] = &resourceInfo
		}
	})
}

func getSchemaVersion(res rschema.Resource) int64 {
	resp := &rschema.SchemaResponse{}
	res.Schema(context.Background(), rschema.SchemaRequest{}, resp)
	contract.Assertf(!resp.Diagnostics.HasError(), "res.Schema() returned error diagnostics: %v", resp.Diagnostics)
	return resp.Schema.Version
}

func runUpgradeTestStatePulumi(t *testing.T, tc upgradeStateTestCase) upgradeStateResult {
	res1 := tc.Resource1
	res2, tracker := instrumentUpgraders(tc.Resource2)

	prov1 := upgradeTestBrigedProvider(t, tc, res1, tc.ResourceInfo1)
	prov2 := upgradeTestBrigedProvider(t, tc, res2, tc.ResourceInfo2)

	tfResourceName := getResourceTypeName(tc.tfProviderName(), res1)

	pm1 := tc.InputsMap1
	if pm1 == nil {
		sch := prov1.P.ResourcesMap().Get(tfResourceName).Schema()
		info := tc.ResourceInfo1.GetFields()
		pm1 = crosstestsimpl.InferPulumiValue(t, sch, info, tc.Inputs1)
	}

	pt := pulcheck.PulCheck(t, prov1, string(upgradeStateYAML(t, tc, prov1, pm1)))

	t.Logf("#### create")
	tracker.phase = createPhase
	createResult := pt.Up(t)
	t.Logf("%s", createResult.StdOut+createResult.StdErr)

	createdState := pt.ExportStack(t)

	schemaVersion1 := getVersionInState(t, createdState)
	require.Equalf(t, getSchemaVersion(tc.Resource1), schemaVersion1, "bad getVersionInState result for create")

	pm2 := tc.InputsMap2
	if pm2 == nil {
		sch := prov2.P.ResourcesMap().Get(tfResourceName).Schema()
		info := tc.ResourceInfo2.GetFields()
		pm2 = crosstestsimpl.InferPulumiValue(t, sch, info, tc.Inputs2)
	}

	p := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
	err := os.WriteFile(p, upgradeStateYAML(t, tc, prov2, pm2), 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")

	handle, err := pulcheck.StartPulumiProvider(context.Background(), prov2)
	require.NoError(t, err)
	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS",
		fmt.Sprintf("%s:%d", tc.tfProviderName(), handle.Port))

	t.Logf("#### refresh")
	tracker.phase = refreshPhase
	refreshResult := pt.Refresh(t)
	t.Logf("%s", refreshResult.StdOut+refreshResult.StdErr)

	schemaVersionR := getVersionInState(t, pt.ExportStack(t))
	t.Logf("schema version after refresh is %d", schemaVersionR)
	require.Equalf(t, getSchemaVersion(tc.Resource2), schemaVersionR, "bad getVersionInState result for refresh")

	// Reset to created state as refresh may have edited it.
	pt.ImportStack(t, createdState)

	t.Logf("#### preview")
	tracker.phase = previewPhase
	previewResult := pt.Preview(t, optpreview.Diff())
	t.Logf("%s", previewResult.StdOut+previewResult.StdErr)

	t.Logf("#### update")
	tracker.phase = updatePhase
	updateResult := pt.Up(t) // --skip-preview would be nice here
	t.Logf("%s", updateResult.StdOut+updateResult.StdErr)

	schemaVersionU := getVersionInState(t, pt.ExportStack(t))
	t.Logf("schema version after update is %d", schemaVersionU)
	if !tc.SkipSchemaVersionAfterUpdateCheck {
		require.Equalf(t, getSchemaVersion(tc.Resource2), schemaVersionU,
			"bad getVersionInState result for update")
	}

	return upgradeStateResult{
		pulumiUpgrades:      tracker.trace,
		pulumiPreviewResult: previewResult,
		pulumiRefreshResult: refreshResult,
		pulumiUpResult:      updateResult,
	}
}

func (tc upgradeStateTestCase) tfProvider(resource rschema.Resource) *upgradeStateTFProvider {
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

func (p *upgradeStateTFProvider) GRPCProvider() tfprotov6.ProviderServer {
	mkProvider := providerserver.NewProtocol6(p)
	return mkProvider()
}

func runUpgradeStateTestTF(t *testing.T, tc upgradeStateTestCase) []upgradeStateTrace {
	t.Logf("Checking TF behavior")

	tfwd := t.TempDir()

	resource1 := tc.Resource1
	resource2, tracker := instrumentUpgraders(tc.Resource2)

	tfd1 := tfcheck.NewTfDriver(t, tfwd, tc.tfProviderName(), tfcheck.NewTFDriverOpts{
		V6Provider: tc.tfProvider(resource1),
	})

	t.Logf("#### create")
	upgradeStateWriteHCL(t, tc, tfwd, resource1, tc.Inputs1)
	tracker.phase = createPhase

	plan, err := tfd1.Plan(t)
	require.NoErrorf(t, err, "tfd1.Plan failed")
	err = tfd1.Apply(t, plan)
	require.NoErrorf(t, err, "tfd1.Apply failed")

	tfd2 := tfcheck.NewTfDriver(t, tfwd, tc.tfProviderName(), tfcheck.NewTFDriverOpts{
		V6Provider: tc.tfProvider(resource2),
	})

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
	t.Logf("%s", plan.StdOut)
	if tc.ExpectFailure {
		require.Errorf(t, err, "plan should have failed")
		return tracker.trace
	}
	require.NoErrorf(t, err, "plan should not have failed")

	t.Logf("#### apply (similar to update)")
	tracker.phase = updatePhase
	err = tfd2.Apply(t, plan)
	require.NoErrorf(t, err, "tfd2.Apply failed")

	return tracker.trace
}

func upgradeStateWriteHCL(t *testing.T, tc upgradeStateTestCase, pwd string, res rschema.Resource, v cty.Value) {
	var buf bytes.Buffer
	tn := getResourceTypeName(tc.tfProviderName(), res)
	hclWriteResource(t, &buf, tn, res, "example", v)
	t.Logf("HCL: %s", buf.String())
	err := os.WriteFile(filepath.Join(pwd, "infra.tf"), buf.Bytes(), 0o600)
	require.NoErrorf(t, err, "Failed to write infra.tf")
}

func upgradeStateYAML(
	t *testing.T,
	tc upgradeStateTestCase,
	info info.Provider,
	resourceConfig resource.PropertyMap,
) []byte {
	tn := getResourceTypeName(tc.tfProviderName(), tc.Resource1)
	token := info.Resources[tn].Tok

	data := map[string]any{
		"name":    "project",
		"runtime": "yaml",
		"backend": map[string]any{
			"url": "file://./data",
		},
	}
	if resourceConfig != nil {
		data["resources"] = map[string]any{
			"example": map[string]any{
				"type": string(token),
				// This is a bit of a leap of faith that serializing PropertyMap
				// to YAML in this way will yield valid Pulumi YAML. This probably
				// needs refinement.
				"properties": crosstests.ConvertResourceValue(t, resourceConfig),
			},
		}
	}

	b, err := yaml.Marshal(data)
	require.NoErrorf(t, err, "marshaling Pulumi.yaml")
	t.Logf("\n\n%s", b)
	return b
}

func skipUnlessLinux(t *testing.T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}

func getResourceTypeName(providerTypeName string, res rschema.Resource) string {
	resp := &rschema.MetadataResponse{}
	res.Metadata(context.Background(), rschema.MetadataRequest{
		ProviderTypeName: providerTypeName,
	}, resp)
	return resp.TypeName
}

// State upgrader implementation that does not do anything.
func nopUpgrade(ctx context.Context, req rschema.UpgradeStateRequest, resp *rschema.UpgradeStateResponse) {
	contract.Assertf(req.State != nil, "expected UpgradeStateRequest with a non-nil State")
	resp.State = *req.State
}
