package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/providerserver"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

const bootDiskSource = "disk-1"

// Regression test for pulumi/pulumi-gcp#3666.
//
// Keep this repro focused on the fields we care about:
//   - boot_disk.source is user input and ignored during the update
//   - Update only changes description, then re-reads from a simulated cloud disk
//   - the cloud disk reports multiple resource_policies, and flattening uses the
//     same guard shape as the real GCP provider
func TestUpdateMaxItemsOneComputedNullStateParity(t *testing.T) {
	t.Parallel()

	resourceUnderTest := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"boot_disk": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"auto_delete": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
						"source": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"initialize_params": {
							Type:     schema.TypeList,
							Optional: true,
							Computed: true,
							ForceNew: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"resource_policies": {
										Type:     schema.TypeList,
										Optional: true,
										Computed: true,
										ForceNew: true,
										MaxItems: 1,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		CreateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
			rd.SetId("id0")
			return readFromCloud(t, rd)
		},
		ReadContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
			return readFromCloud(t, rd)
		},
		UpdateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
			return readFromCloud(t, rd)
		},
	}

	tfErr := runTerraformUpdate(t, resourceUnderTest)
	pulumiErr := runPulumiUpdate(t, resourceUnderTest)

	require.Equalf(t, tfErr == nil, pulumiErr == nil,
		"terraform update error = %v, pulumi update error = %v",
		tfErr, pulumiErr)
}

func readFromCloud(t *testing.T, rd *schema.ResourceData) diag.Diagnostics {
	t.Helper()

	require.NoError(t, rd.Set("boot_disk", flattenBootDiskFromCloud(t, rd)))
	return nil
}

func flattenBootDiskFromCloud(t *testing.T, rd *schema.ResourceData) []interface{} {
	t.Helper()

	cloudPolicies := cloudBootDiskResourcePolicies(rd)
	resourcePolicies0 := rd.Get("boot_disk.0.initialize_params.0.resource_policies.0")
	resourcePolicies := rd.Get("boot_disk.0.initialize_params.0.resource_policies")
	t.Logf(
		"flatten guard values: resource_policies.0=%#v resource_policies=%#v cloud=%#v",
		resourcePolicies0, resourcePolicies, cloudPolicies,
	)

	cloudPolicies = append([]string(nil), cloudPolicies...)
	if resourcePolicies0 == nil || resourcePolicies == nil {
		cloudPolicies = nil
	}

	return []interface{}{
		map[string]interface{}{
			"auto_delete": false,
			"source":      bootDiskSource,
			"initialize_params": []interface{}{
				map[string]interface{}{
					"resource_policies": cloudPolicies,
				},
			},
		},
	}
}

func cloudBootDiskResourcePolicies(rd *schema.ResourceData) []string {
	if rd.Get("description").(string) == "description 2" {
		return []string{"policy-1", "policy-2"}
	}
	return nil
}

func runTerraformUpdate(t *testing.T, resourceUnderTest *schema.Resource) error {
	t.Helper()

	tfd := tfcheck.NewTfDriver(t, t.TempDir(), "crossprovider", tfcheck.NewTFDriverOpts{
		SDKProvider: &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"crossprovider_test_res": resourceUnderTest,
			},
		},
	})

	tfd.Write(t, terraformProgram("description 1"))
	plan, err := tfd.Plan(t)
	require.NoError(t, err)
	require.NoError(t, tfd.ApplyPlan(t, plan))

	tfd.Write(t, terraformProgram("description 2"))
	plan, err = tfd.Plan(t)
	if err != nil {
		return err
	}
	return tfd.ApplyPlan(t, plan)
}

func runPulumiUpdate(t *testing.T, resourceUnderTest *schema.Resource) error {
	t.Helper()

	bridgedProvider := pulcheck.BridgedProvider(t, "crossprovider", &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"crossprovider_test_res": resourceUnderTest,
		},
	}, pulcheck.WithResourceInfo(map[string]*info.Resource{
		"crossprovider_test_res": {Tok: "crossprovider:index:TestRes"},
	}))

	pt := patchedPulumiTest(t, bridgedProvider, pulumiProgram("description 1"))
	pt.Up(t)
	pt.WritePulumiYaml(t, pulumiProgram("description 2"))

	res, err := pt.CurrentStack().Up(pt.Context())
	t.Logf("pulumi up stdout:\n%s", res.StdOut)
	t.Logf("pulumi up stderr:\n%s", res.StdErr)
	return err
}

func terraformProgram(description string) string {
	return fmt.Sprintf(`
resource "crossprovider_test_res" "example" {
  description = %q

  boot_disk {
    auto_delete = false
    source      = %q
  }

  lifecycle {
    ignore_changes = [boot_disk]
  }
}
`, description, bootDiskSource)
}

func pulumiProgram(description string) string {
	return fmt.Sprintf(`
name: project
runtime: yaml
backend:
  url: file://./data
resources:
  example:
    type: crossprovider:index:TestRes
    properties:
      description: %q
      bootDisk:
        autoDelete: false
        source: %q
    options:
      ignoreChanges:
        - bootDisk
`, description, bootDiskSource)
}

func patchedPulumiTest(t *testing.T, bridgedProvider info.Provider, program string) *pulumitest.PulumiTest {
	t.Helper()

	puwd := t.TempDir()
	program = strings.ReplaceAll(program, "\t", "    ")
	err := os.WriteFile(filepath.Join(puwd, "Pulumi.yaml"), []byte(program), 0o600)
	require.NoError(t, err)

	return pulumitest.NewPulumiTest(t, puwd,
		opttest.Env("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			bridgedProvider.Name,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				prov, err := pulcheck.ProviderServerFromInfo(ctx, bridgedProvider)
				if err != nil {
					return 0, err
				}
				prepareProviderForBridgePanics(t, prov)

				handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
					Init: func(srv *grpc.Server) error {
						pulumirpc.RegisterResourceProviderServer(srv, prov)
						return nil
					},
				})
				if err != nil {
					return 0, err
				}
				return providers.Port(handle.Port), nil
			},
		),
	)
}

func prepareProviderForBridgePanics(t *testing.T, server pulumirpc.ResourceProviderServer) {
	t.Helper()

	wrapped, ok := server.(*providerserver.PanicRecoveringProviderServer)
	require.True(t, ok, "expected panic-recovering provider server")

	setPanicRecoveringProviderServerField(t, wrapped, "logger", logging.NewDiscardSink())
	setPanicRecoveringProviderServerField(t, wrapped, "omitStackTraces", true)
}

func setPanicRecoveringProviderServerField(
	t *testing.T,
	server *providerserver.PanicRecoveringProviderServer,
	fieldName string,
	value any,
) {
	t.Helper()

	field := reflect.ValueOf(server).Elem().FieldByName(fieldName)
	require.True(t, field.IsValid(), "missing field %q", fieldName)

	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}
