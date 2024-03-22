package crosstests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	pulumidiag "github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type diffTestCase struct {
	// Schema for the resource to test diffing on.
	Resource *schema.Resource

	// Two resource configurations. The representation assumes JSON Configuration Syntax
	// accepted by TF, that is, these values when serialized with JSON should parse as .tf.json
	// files. If Config1 is nil, assume a Create flow. If Config2 is nil, assume a Delete flow.
	// Otherwise assume an Update flow for a resource.
	//
	// See	https://developer.hashicorp.com/terraform/language/syntax/json
	Config1, Config2 any

	// Bypass interacting with the bridged Pulumi provider.
	SkipPulumi bool
}

const (
	providerShortName = "crossprovider"
	rtype             = "crossprovider_testres"
	rtok              = "TestRes"
	rtoken            = providerShortName + ":index:" + rtok
	providerName      = "registry.terraform.io/hashicorp/" + providerShortName
	providerVer       = "0.0.1"
)

func runDiffCheck(t *testing.T, tc diffTestCase) {
	// ctx := context.Background()
	tfwd := t.TempDir()

	reattachConfig := startTFProvider(t, tc)

	tfWriteJSON(t, tfwd, tc.Config1)
	p1 := runTFPlan(t, tfwd, reattachConfig)
	runTFApply(t, tfwd, reattachConfig, p1)

	tfWriteJSON(t, tfwd, tc.Config2)
	p2 := runTFPlan(t, tfwd, reattachConfig)
	runTFApply(t, tfwd, reattachConfig, p2)

	{
		planBytes, err := json.MarshalIndent(p2.RawPlan, "", "  ")
		contract.AssertNoErrorf(err, "failed to marshal terraform plan")
		t.Logf("TF plan: %v", string(planBytes))
	}

	if tc.SkipPulumi {
		return
	}

	puwd := t.TempDir()
	pulumiWriteYaml(t, tc, puwd, tc.Config1)

	pt := pulumitest.NewPulumiTest(t, puwd,
		// Needed while using Nix-built pulumi.
		opttest.Env("PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			providerShortName,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle, err := startPulumiProvider(ctx, tc)
				require.NoError(t, err)
				return providers.Port(handle.Port), nil
			},
		),
	)

	pt.Up()

	pulumiWriteYaml(t, tc, puwd, tc.Config2)
	x := pt.Up()

	verifyBasicDiffAgreement(t, p2, x.Summary)
}

func tfWriteJSON(t *testing.T, cwd string, rconfig any) {
	config := map[string]any{
		"resource": map[string]any{
			rtype: map[string]any{
				"example": rconfig,
			},
		},
	}
	config1bytes, err := json.MarshalIndent(config, "", "  ")
	require.NoErrorf(t, err, "serializing test.tf.json")
	err = os.WriteFile(filepath.Join(cwd, "test.tf.json"), config1bytes, 0600)
	require.NoErrorf(t, err, "writing test.tf.json")
}

type tfPlan struct {
	PlanFile string
	RawPlan  any
}

func (*tfPlan) OpType() *apitype.OpType {
	return nil
}

func runTFPlan(t *testing.T, cwd string, reattachConfig *plugin.ReattachConfig) tfPlan {
	planFile := filepath.Join(cwd, "test.tfplan")
	env := []string{formatReattachEnvVar(providerName, reattachConfig)}
	execCmd(t, cwd, env, "terraform", "plan", "-refresh=false", "-out", planFile)

	cmd := execCmd(t, cwd, env, "terraform", "show", "-json", planFile)
	tp := tfPlan{PlanFile: planFile}
	err := json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &tp.RawPlan)
	contract.AssertNoErrorf(err, "failed to unmarshal terraform plan")
	return tp
}

func runTFApply(t *testing.T, cwd string, reattachConfig *plugin.ReattachConfig, p tfPlan) {
	execCmd(t, cwd, []string{formatReattachEnvVar(providerName, reattachConfig)},
		"terraform", "apply", "-auto-approve", "-refresh=false", p.PlanFile)
}

func toTFProvider(tc diffTestCase) *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			rtype: tc.Resource,
		},
	}
}

func startTFProvider(t *testing.T, tc diffTestCase) *plugin.ReattachConfig {
	tc.Resource.CustomizeDiff = func(
		ctx context.Context, rd *schema.ResourceDiff, i interface{},
	) error {
		// fmt.Printf(`\n\n   CustomizeDiff: rd.Get("set") ==> %#v\n\n\n`, rd.Get("set"))
		// fmt.Println("\n\nGetRawPlan:   ", rd.GetRawPlan().GoString())
		// fmt.Println("\n\nGetRawConfig: ", rd.GetRawConfig().GoString())
		// fmt.Println("\n\nGetRawState:  ", rd.GetRawState().GoString())
		return nil
	}

	if tc.Resource.DeleteContext == nil {
		tc.Resource.DeleteContext = func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			return diag.Diagnostics{}
		}
	}

	if tc.Resource.CreateContext == nil {
		tc.Resource.CreateContext = func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			rd.SetId("newid")
			return diag.Diagnostics{}
		}
	}

	tc.Resource.UpdateContext = func(
		ctx context.Context, rd *schema.ResourceData, i interface{},
	) diag.Diagnostics {
		//fmt.Printf(`\n\n   Update: rd.Get("set") ==> %#v\n\n\n`, rd.Get("set"))
		return diag.Diagnostics{}
	}

	p := toTFProvider(tc)

	serverFactory := func() tfprotov5.ProviderServer {
		return p.GRPCProvider()
	}

	ctx := context.Background()

	reattachConfigCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})

	serveOpts := []tf5server.ServeOpt{
		tf5server.WithDebug(ctx, reattachConfigCh, closeCh),
		tf5server.WithLoggingSink(t),
	}

	go func() {
		err := tf5server.Serve(providerName, serverFactory, serveOpts...)
		require.NoError(t, err)
	}()

	reattachConfig := <-reattachConfigCh
	return reattachConfig
}

func formatReattachEnvVar(name string, pluginReattachConfig *plugin.ReattachConfig) string {
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

func TestSimpleStringNoChange(t *testing.T) {
	skipUnlessLinux(t)
	runDiffCheck(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {

					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		Config1: map[string]any{
			"name": "A",
		},
		Config2: map[string]any{
			"name": "A",
		},
	})
}

func TestSimpleStringRename(t *testing.T) {
	skipUnlessLinux(t)
	runDiffCheck(t, diffTestCase{
		Resource: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		Config1: map[string]any{
			"name": "A",
		},
		Config2: map[string]any{
			"name": "B",
		},
	})
}

func TestSetReordering(t *testing.T) {
	skipUnlessLinux(t)
	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"set": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		CreateContext: func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			rd.SetId("newid")
			require.IsType(t, &schema.Set{}, rd.Get("set"))
			return diag.Diagnostics{}
		},
	}
	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1: map[string]any{
			"set": []string{"A", "B"},
		},
		Config2: map[string]any{
			"set": []string{"B", "A"},
		},
	})
}

func TestAws2442(t *testing.T) {
	skipUnlessLinux(t)
	hashes := map[int]string{}

	stringHashcode := func(s string) int {
		v := int(crc32.ChecksumIEEE([]byte(s)))
		if v >= 0 {
			return v
		}
		if -v >= 0 {
			return -v
		}
		// v == MinInt
		return 0
	}

	resourceParameterHash := func(v interface{}) int {
		var buf bytes.Buffer
		m := v.(map[string]interface{})
		// Store the value as a lower case string, to match how we store them in FlattenParameters
		name := strings.ToLower(m["name"].(string))
		buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(m["name"].(string))))
		buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(m["apply_method"].(string))))
		buf.WriteString(fmt.Sprintf("%s-", m["value"].(string)))

		// This hash randomly affects the "order" of the set, which affects in what order parameters
		// are applied, when there are more than 20 (chunked).
		n := stringHashcode(buf.String())

		if old, ok := hashes[n]; ok {
			if old != name {
				panic("Hash collision on " + name)
			}
		}
		hashes[n] = name
		//fmt.Println("setting hash name", n, name)
		return n
	}

	rschema := map[string]*schema.Schema{
		"parameter": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"apply_method": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "immediate",
					},
					"name": {
						Type:     schema.TypeString,
						Required: true,
					},
					"value": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
			Set: resourceParameterHash,
		},
	}
	resource := &schema.Resource{
		Schema: rschema,
		CreateContext: func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			rd.SetId("someid") // CreateContext must pick an ID
			parameterList := rd.Get("parameter").(*schema.Set).List()
			slices.Reverse(parameterList)
			// Now intentionally reorder parameters away from the canonical order.
			err := rd.Set("parameter", parameterList[0:3])
			require.NoError(t, err)
			fmt.Println("CREATE! set to 3")
			return make(diag.Diagnostics, 0)
		},
		// UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
		// 	panic("UPD")
		// },
	}

	type parameter struct {
		name        string
		value       string
		applyMethod string
	}

	parameters := []parameter{
		{
			name:        "max_connections",
			value:       "500",
			applyMethod: "pending-reboot",
		},
		{
			name:        "wal_buffers",
			value:       "2048",
			applyMethod: "pending-reboot",
		}, // in 8kB
		{
			name:        "default_statistics_target",
			value:       "100",
			applyMethod: "immediate",
		},
		{
			name:        "random_page_cost",
			value:       "1.1",
			applyMethod: "immediate",
		},
		{
			name:        "effective_io_concurrency",
			value:       "200",
			applyMethod: "immediate",
		},
		{
			name:        "work_mem",
			value:       "65536",
			applyMethod: "immediate",
		}, // in kB
		{
			name:        "max_parallel_workers_per_gather",
			value:       "4",
			applyMethod: "immediate",
		},
		{
			name:        "max_parallel_maintenance_workers",
			value:       "4",
			applyMethod: "immediate",
		},
		{
			name:        "pg_stat_statements.track",
			value:       "ALL",
			applyMethod: "immediate",
		},
		{
			name:        "shared_preload_libraries",
			value:       "pg_stat_statements,auto_explain",
			applyMethod: "pending-reboot",
		},
		{
			name:        "track_io_timing",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_min_duration_statement",
			value:       "1000",
			applyMethod: "immediate",
		},
		{
			name:        "log_lock_waits",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_temp_files",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_checkpoints",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_connections",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_disconnections",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "log_autovacuum_min_duration",
			value:       "0",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_format",
			value:       "json",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_min_duration",
			value:       "1000",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_analyze",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_buffers",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_timing",
			value:       "0",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_triggers",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_verbose",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.log_nested_statements",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "auto_explain.sample_rate",
			value:       "1",
			applyMethod: "immediate",
		},
		{
			name:        "rds.logical_replication",
			value:       "1",
			applyMethod: "pending-reboot",
		},
	}

	jsonifyParameters := func(parameters []parameter) []map[string]interface{} {
		var result []map[string]interface{}
		for _, p := range parameters {
			result = append(result, map[string]interface{}{
				"name":         p.name,
				"value":        p.value,
				"apply_method": p.applyMethod,
			})
		}
		return result
	}

	cfg := map[string]any{
		"parameter": jsonifyParameters(parameters),
	}

	ps := jsonifyParameters(parameters)
	slices.Reverse(ps)
	cfg2 := map[string]any{
		"parameter": ps,
	}

	runDiffCheck(t, diffTestCase{
		Resource: resource,
		Config1:  cfg,
		Config2:  cfg2,
	})
}

func toPulumiProvider(tc diffTestCase) tfbridge.ProviderInfo {
	return tfbridge.ProviderInfo{
		Name: providerShortName,

		P: shimv2.NewProvider(toTFProvider(tc), shimv2.WithPlanResourceChange(
			func(tfResourceType string) bool { return true },
		)),

		Resources: map[string]*tfbridge.ResourceInfo{
			rtype: {
				Tok: rtoken,
			},
		},
	}
}

func startPulumiProvider(
	ctx context.Context,
	tc diffTestCase,
) (*rpcutil.ServeHandle, error) {
	info := toPulumiProvider(tc)

	sink := pulumidiag.DefaultSink(io.Discard, io.Discard, pulumidiag.FormatOptions{
		Color: colors.Never,
	})

	schema, err := tfgen.GenerateSchema(info, sink)
	if err != nil {
		return nil, fmt.Errorf("tfgen.GenerateSchema failed: %w", err)
	}

	schemaBytes, err := json.MarshalIndent(schema, "", " ")
	if err != nil {
		return nil, fmt.Errorf("json.MarshalIndent(schema, ..) failed: %w", err)
	}

	prov := tfbridge.NewProvider(ctx, nil, providerShortName, providerVer, info.P, info, schemaBytes)

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("rpcutil.ServeWithOptions failed: %w", err)
	}

	return &handle, nil
}

func pulumiWriteYaml(t *testing.T, tc diffTestCase, puwd string, tfConfig any) {
	schema := sdkv2.NewResource(tc.Resource).Schema()
	pConfig, err := convertConfigToPulumi(schema, nil, tfConfig)
	require.NoErrorf(t, err, "convertConfigToPulumi failed")
	data := map[string]any{
		"name":    "project",
		"runtime": "yaml",
		"resources": map[string]any{
			"example": map[string]any{
				"type":       fmt.Sprintf("%s:index:%s", providerShortName, rtok),
				"properties": pConfig,
			},
		},
		"backend": map[string]any{
			"url": "file://./data",
		},
	}
	b, err := yaml.Marshal(data)
	require.NoErrorf(t, err, "marshaling Pulumi.yaml")
	p := filepath.Join(puwd, "Pulumi.yaml")
	err = os.WriteFile(p, b, 0600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")
}

func execCmd(t *testing.T, wdir string, environ []string, program string, args ...string) *exec.Cmd {
	t.Logf("%s %s", program, strings.Join(args, " "))
	cmd := exec.Command(program, args...)
	var stdout, stderr bytes.Buffer
	cmd.Dir = wdir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, environ...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(t, err, "error from `%s %s`\n\nStdout:\n%s\n\nStderr:\n%s\n\n",
		program, strings.Join(args, " "), stdout.String(), stderr.String())
	return cmd
}

func convertConfigToPulumi(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	tfConfig any,
) (any, error) {
	objectType := convert.InferObjectType(schemaMap, nil)
	bytes, err := json.Marshal(tfConfig)
	if err != nil {
		return nil, err
	}
	// Knowingly using a deprecated function so we can connect back up to tftypes.Value; if this disappears it
	// should not be prohibitively difficult to rewrite or vendor.
	//
	//nolint:staticcheck
	v, err := tftypes.ValueFromJSON(bytes, objectType)
	if err != nil {
		return nil, err
	}
	decoder, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   schemaMap,
		SchemaInfos: schemaInfos,
		Object:      &objectType,
	})
	if err != nil {
		return nil, err
	}
	pm, err := convert.DecodePropertyMap(decoder, v)
	if err != nil {
		return nil, err
	}
	return pm.Mappable(), nil
}

// Still discovering the structure of JSON-serialized TF plans. The information required from these is, primarily, is
// whether the resource is staying unchanged, being updated or replaced. Secondarily, would be also great to know
// detailed paths of properties causing the change, though that is more difficult to cross-compare with Pulumi.
//
// For now this is code is similar to `jq .resource_changes[0].change.actions[0] plan.json`.
func parseChangesFromTFPlan(plan tfPlan) string {
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
	contract.Assertf(len(actions) == 1, "expected exactly one action")
	return actions[0]
}

func verifyBasicDiffAgreement(t *testing.T, plan tfPlan, us auto.UpdateSummary) {
	t.Logf("UpdateSummary.ResourceChanges: %#v", us.ResourceChanges)
	tfAction := parseChangesFromTFPlan(plan)
	switch tfAction {
	case "update":
		require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
		rc := *us.ResourceChanges
		assert.Equalf(t, 1, rc[string(apitype.OpSame)], "expected one resource to stay the same - the stack")
		assert.Equalf(t, 1, rc[string(apitype.Update)], "expected the test resource to get an update plan")
		assert.Equalf(t, 2, len(rc), "expected two entries in UpdateSummary.ResourceChanges")
	case "no-op":
		require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
		rc := *us.ResourceChanges
		assert.Equalf(t, 2, rc[string(apitype.OpSame)], "expected the test resource and the stack to stay the same")
		assert.Equalf(t, 1, len(rc), "expected one entry in UpdateSummary.ResourceChanges")
	default:
		panic("TODO: do not understand this TF action yet: " + tfAction)
	}
}

func skipUnlessLinux(t *testing.T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}
