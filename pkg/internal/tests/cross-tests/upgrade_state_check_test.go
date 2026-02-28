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

package crosstests

import (
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

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// Verify state upgrade interaction compatibility on schema change.
//
// Play-by-play:
//
//  1. provision Inputs1 with the provider based on Resource1 schema
//  2. build the provider based on Resource2 schema
//  3. refresh with the Resource2 provider
//  4. update to Inputs2 with the Resource2 provider
type upgradeStateTestCase struct {
	Resource1     *schema.Resource
	ResourceInfo1 *info.Resource
	Inputs1       any
	InputsMap1    resource.PropertyMap // if nil, best-effort inferred from TF-shaped Inputs1
	Resource2     *schema.Resource
	ResourceInfo2 *info.Resource
	Inputs2       any
	InputsMap2    resource.PropertyMap // if nil, best-effort inferred from TF-shaped Inputs2

	ExpectFailure        bool // expect the test to fail, as in downgrades
	ExpectedRawStateType cty.Type

	SkipPulumi                        string // Reason to skip all Pulumi parts of the test
	SkipSchemaVersionAfterUpdateCheck bool

	// Turning this on would check what would happen if `pulumi refresh` ran a Resource2-style provider against a
	// Resource1-style state. This is not currently how Pulumi works in production though, as `pulumi refresh`
	// would always pick the provider version recorded in the state to perform operations against. So these checks
	// are possibly moot, but might be useful in the future if Pulumi behavior around refresh changes.
	ExperimentalPulumiRefresh bool

	// Do not auto-generate an Update function to test resources that do not support Update.
	DoesNotSupportUpdate bool
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
	RawState any                   // RawState passed to the StateUpgrader
	Meta     any                   // Meta passed to the StateUpgrader
	Result   map[string]any        // Result returned by the StateUpgrader
	Err      error                 // Error returned by the StateUpgrader
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
		tcTF := instrumentCustomizeDiff(t, tc)
		tcTF = instrumentUpdate(t, tcTF)
		result.tfUpgrades = runUpgradeStateTestTF(t, tcTF)
	})

	t.Run("pulumi", func(t *testing.T) {
		if tc.SkipPulumi != "" {
			t.Skip(tc.SkipPulumi)
		}
		tcPulumi := instrumentCustomizeDiff(t, tc)
		tcPulumi = instrumentUpdate(t, tcPulumi)
		resultPulumi := runUpgradeTestStatePulumi(t, tcPulumi)
		result.pulumiUpgrades = resultPulumi.pulumiUpgrades
		result.pulumiRefreshResult = resultPulumi.pulumiRefreshResult
		result.pulumiPreviewResult = resultPulumi.pulumiPreviewResult
		result.pulumiUpResult = resultPulumi.pulumiUpResult
	})

	return result
}

// Making sure the RawState() received is of an appropriate type.
func checkRawState(t *testing.T, tc upgradeStateTestCase, receivedRawState cty.Value, method string) {
	if tc.ExpectedRawStateType.GoString() == cty.NilType.GoString() {
		return
	}
	receivedValueType := receivedRawState.Type()
	assert.Truef(t, tc.ExpectedRawStateType.Equals(receivedValueType),
		"%s expected GetRawState().Type() be %s, got %s; the value is %v",
		method,
		tc.ExpectedRawStateType.GoString(),
		receivedValueType.GoString(),
		receivedRawState.GoString())
}

func instrumentCustomizeDiff(t *testing.T, tc upgradeStateTestCase) upgradeStateTestCase {
	counter := new(atomic.Int32)

	r2 := *tc.Resource2
	require.Nilf(t, r2.CustomizeDiff, "Resource2.CustomizeDiff cannot yet be set in tests")

	r2.CustomizeDiff = func(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {
		counter.Add(1)
		checkRawState(t, tc, rd.GetRawState(), "CustomizeDiff")
		return nil
	}
	tc.Resource2 = &r2

	if !tc.ExpectFailure {
		t.Cleanup(func() {
			n := counter.Load()
			assert.Truef(t, n > 0, "expected CustomizeDiff to be called at least once, got %d calls", n)
		})
	}
	return tc
}

func instrumentUpdate(t *testing.T, tc upgradeStateTestCase) upgradeStateTestCase {
	if tc.DoesNotSupportUpdate {
		r1 := *tc.Resource1
		require.Nilf(t, r1.UpdateContext, "Resource1.UpdateContext is not compatible with DoesNotSupportUpdate")
		r2 := *tc.Resource2
		require.Nilf(t, r2.UpdateContext, "Resource2.UpdateContext is not compatible with DoesNotSupportUpdate")

		return tc
	}

	r2 := *tc.Resource2
	require.Nilf(t, r2.UpdateContext, "Resource2.UpdateContext cannot yet be set in tests")

	r2.UpdateContext = func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
		checkRawState(t, tc, rd.GetRawState(), "UpdateContext")
		return nil
	}
	tc.Resource2 = &r2
	return tc
}

type upgraderTracker struct {
	phase upgradeStateTestPhase
	trace []upgradeStateTrace
	mu    sync.Mutex
}

func (t *upgraderTracker) instrumentUpgrader(i int, u schema.StateUpgrader) schema.StateUpgrader {
	upgrade := u.Upgrade
	return schema.StateUpgrader{
		Version: u.Version,
		Type:    u.Type,
		Upgrade: func(
			ctx context.Context,
			rawState map[string]interface{},
			meta interface{},
		) (map[string]interface{}, error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			ret, err := upgrade(ctx, rawState, meta)
			t.trace = append(t.trace, upgradeStateTrace{
				Phase:    t.phase,
				Upgrader: i,
				RawState: rawState,
				Meta:     meta,
				Result:   ret,
				Err:      err,
			})
			return ret, err
		},
	}
}

func instrumentUpgraders(s *schema.Resource) (*schema.Resource, *upgraderTracker) {
	tr := &upgraderTracker{}
	copy := *s
	copy.StateUpgraders = make([]schema.StateUpgrader, len(s.StateUpgraders))
	for i, u := range s.StateUpgraders {
		copy.StateUpgraders[i] = tr.instrumentUpgrader(i, u)
	}
	return &copy, tr
}

func getVersionInState(t T, stack apitype.UntypedDeployment) int {
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

func upgradeTestBrigedProvider(t T, r *schema.Resource, ri *info.Resource) info.Provider {
	tfp := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: r}}
	p := pulcheck.BridgedProvider(t, defProviderShortName, tfp)
	if ri != nil {
		resourceInfo := *ri
		resourceInfo.Tok = p.Resources[defRtype].Tok
		p.Resources[defRtype] = &resourceInfo
	}
	return p
}

func runUpgradeTestStatePulumi(t T, tc upgradeStateTestCase) upgradeStateResult {
	res1 := tc.Resource1
	res2, tracker := instrumentUpgraders(tc.Resource2)

	prov1 := upgradeTestBrigedProvider(t, res1, tc.ResourceInfo1)
	prov2 := upgradeTestBrigedProvider(t, res2, tc.ResourceInfo2)

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
	}

	inputs1 := coalesceInputs(t, tc.Resource1.Schema, tc.Inputs1)
	inputs2 := coalesceInputs(t, tc.Resource2.Schema, tc.Inputs2)

	pm1 := tc.InputsMap1
	if pm1 == nil {
		sch := prov1.P.ResourcesMap().Get(pd.tfResourceName).Schema()
		info := tc.ResourceInfo1.GetFields()
		pm1 = crosstestsimpl.InferPulumiValue(t, sch, info, inputs1)
	}

	yamlProgram := pd.generateYAML(t, pm1)
	pt := pulcheck.PulCheck(t, prov1, string(yamlProgram))

	t.Logf("#### create")
	tracker.phase = createPhase
	createResult := pt.Up(t)
	t.Logf("%s", createResult.StdOut+createResult.StdErr)

	createdState := pt.ExportStack(t)

	t.Logf("createdState: %v", string(createdState.Deployment))

	schemaVersion1 := getVersionInState(t, createdState)
	require.Equalf(t, tc.Resource1.SchemaVersion, schemaVersion1, "bad getVersionInState result for create")

	pm2 := tc.InputsMap2
	if pm2 == nil {
		sch := prov1.P.ResourcesMap().Get(pd.tfResourceName).Schema()
		info := tc.ResourceInfo2.GetFields()
		pm2 = crosstestsimpl.InferPulumiValue(t, sch, info, inputs2)
	}

	yamlProgram = pd.generateYAML(t, pm2)
	p := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
	err := os.WriteFile(p, yamlProgram, 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")

	handle, err := pulcheck.StartPulumiProvider(context.Background(), prov2)
	require.NoError(t, err)
	pt.CurrentStack().Workspace().SetEnvVar("PULUMI_DEBUG_PROVIDERS",
		fmt.Sprintf("%s:%d", defProviderShortName, handle.Port))

	var refreshResult auto.RefreshResult
	if tc.ExperimentalPulumiRefresh {
		t.Logf("#### refresh")
		tracker.phase = refreshPhase
		refreshResult = pt.Refresh(t)
		t.Logf("%s", refreshResult.StdOut+refreshResult.StdErr)

		schemaVersionR := getVersionInState(t, pt.ExportStack(t))
		t.Logf("schema version after refresh is %d", schemaVersionR)
		require.Equalf(t, tc.Resource2.SchemaVersion, schemaVersionR,
			"bad getVersionInState result for refresh")
	}

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
		require.Equalf(t, tc.Resource2.SchemaVersion, schemaVersionU,
			"bad getVersionInState result for update")
	}

	return upgradeStateResult{
		pulumiUpgrades:      tracker.trace,
		pulumiPreviewResult: previewResult,
		pulumiRefreshResult: refreshResult,
		pulumiUpResult:      updateResult,
	}
}

func runUpgradeStateTestTF(t T, tc upgradeStateTestCase) []upgradeStateTrace {
	t.Logf("Checking TF behavior")
	rname := "example"
	inputs1 := coalesceInputs(t, tc.Resource1.Schema, tc.Inputs1)
	inputs2 := coalesceInputs(t, tc.Resource2.Schema, tc.Inputs2)
	resource2, tracker := instrumentUpgraders(tc.Resource2)
	tfwd := t.TempDir()

	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource1)

	t.Logf("#### create")
	tracker.phase = createPhase
	_ = tfd.writePlanApply(t, tc.Resource1.Schema, defRtype, rname, inputs1, lifecycleArgs{})

	tfd2 := newTFResDriver(t, tfwd, defProviderShortName, defRtype, resource2)

	t.Logf("#### save current state as created state")
	createdState, err := os.ReadFile(filepath.Join(tfwd, "terraform.tfstate"))
	require.NoError(t, err)

	t.Logf("#### refresh")
	tracker.phase = refreshPhase
	err = tfd2.refreshErr(t, tc.Resource2.Schema, defRtype, rname, inputs2, lifecycleArgs{})
	if tc.ExpectFailure {
		require.Errorf(t, err, "refresh should have failed")
	} else {
		require.NoErrorf(t, err, "refresh should not have failed")
	}

	// Reset the state to the created state check apply, as refresh has migrated the state.
	t.Logf("#### reset state to created state")
	err = os.WriteFile(filepath.Join(tfwd, "terraform.tfstate"), createdState, 0o600)
	require.NoError(t, err)

	t.Logf("#### plan (similar to preview)")
	tracker.phase = previewPhase
	plan, err := tfd2.writePlanErr(t, tc.Resource2.Schema, defRtype, rname, inputs2, lifecycleArgs{})
	if tc.ExpectFailure {
		require.Errorf(t, err, "plan should have failed")
		return tracker.trace
	}
	require.NoErrorf(t, err, "plan should not have failed")
	t.Logf("plan.StdOut: %s", plan.StdOut)

	t.Logf("#### apply (similar to update)")
	tracker.phase = updatePhase
	applyStdout, err := tfd2.driver.ApplyPlanReturnStdOut(t, plan)
	require.NoError(t, err)
	t.Logf("applyStdout: %s", string(applyStdout))

	return tracker.trace
}

func nopUpgrade(
	ctx context.Context,
	rawState map[string]interface{},
	meta interface{},
) (map[string]interface{}, error) {
	return rawState, nil
}

func skipUnlessLinux(t T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}
