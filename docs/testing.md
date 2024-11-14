# Testing in the Pulumi Terraform Bridge

The Pulumi Terraform Bridge has a few different testing frameworks. This documents attempts to help developers understand how to best test their features.

## Integration Tests Using a Terraform Schema and a Pulumi YAML Program

The preferred way to test new features and reproduce bugs is to author an integration test using a Terraform schema and a Pulumi YAML program. This is the best way to test end-to-end scenarios, including multi-step workflows.
The two bridge frameworks (The Plugin SDK and the Plugin Framework) have slight differences in the APIs but the core concepts are the same.

We need:
1. A Terraform provider schema - this is the way Terraform providers are authored and allows us to extract only the relevant parts of the Terraform provider code.
1. A Pulumi program - this is the way Pulumi programs are authored and allows us to test the bridged provider through a minimal Pulumi program.
1. A set of pulumi operations performed on the program - these are meant to simulate a real user workflow, which either exercises the feature under test or reproduces the bug.

These tests use the [`pulumiTest` library](https://github.com/pulumi/providertest/tree/main/pulumitest) to run the Pulumi program.

### Example from the Plugin SDK

From [./pkg/tests/schema_pulumi_test.go](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/tests/schema_pulumi_test.go):

```go
import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
)

func TestBasic(t *testing.T) {
	t.Parallel()
	tfResourceMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}
	tfProvider := &schema.Provider{ResourcesMap: tfResourceMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfProvider)
	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
	  test: "hello"
outputs:
  testOut: ${mainRes.test}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	upResult := pt.Up(t)
	require.Equal(t, "hello", upResult.Outputs["testOut"].Value)
}
```

#### Explanation

`tfResourceMap` is a map of Terraform resource types to their schemas. This can be adapted from an existing Terraform provider or created ad-hoc. `tfProvider` is the Terraform provider which uses `tfResourceMap` to construct the provider.

`bridgedProvider` is a wrapper around a Terraform provider that has been instrumented to work with Pulumi. The `pulcheck` package provides a helper to create a bridged provider.

`program` is a Pulumi program that uses the bridged provider. In this example, it creates a single resource and exports its output.

`pt` is an instance of the [`pulumiTest` library](https://github.com/pulumi/providertest/tree/main/pulumitest) which uses Automation API to run the Pulumi program. It is returned by the `PulCheck` helper, which is a wrapper around the `pulumiTest` library specifically for testing bridged providers.

`upResult` is the result of running the Pulumi program. It contains the outputs of the program as well as GRPC logs and other useful information which can help us assert that the program behaves as expected.


### Example from the Plugin Framework

From [./pkg/pf/tests/schema_and_program_test.go](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/pf/tests/schema_and_program_test.go):

```go
import (
	"testing"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/providerbuilder"
)

func TestBasic(t *testing.T) {
    t.Parallel()
	provBuilder := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllResources: []providerbuilder.Resource{
				{
					Name: "test",
					ResourceSchema: rschema.Schema{
						Attributes: map[string]rschema.Attribute{
							"s": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
		})

	prov := bridgedProvider(provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
        properties:
            s: "hello"
outputs:
  testOut: ${mainRes.s}`

	pt, err := pulcheck.PulCheck(t, prov, program)
	require.NoError(t, err)

	upResult := pt.Up(t)
	require.Equal(t, "hello", upResult.Outputs["testOut"].Value)
}
```

#### Explanation

The Plugin Framework uses a different set of helpers to create a bridged provider but the core concepts are the same.

`provBuilder` is a helper to create a PF Terraform provider from a set of resources. The TF API is a bit more involved than the SDKv2 API, so we have a separate helper for that.

`prov` is the bridged provider. This uses the `pulcheck` helper to create a bridged provider. Notice that `pf` has a separate `pulcheck` helper.

`program` is a Pulumi program that uses the bridged provider. In this example, it creates a single resource and exports its output.

`pt` is an instance of the [`pulumiTest` library](https://github.com/pulumi/providertest/tree/main/pulumitest).


## Cross-Tests

Another useful framework for testing the bridge are the cross-tests. These are integration tests that run a Terraform provider both through the Terraform CLI and through the Pulumi Bridge and then compare the outputs of the two. There is a detailed explanation of the specifics of cross-tests in the [cross-tests README](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/internal/tests/cross-tests/README.md) and the [pf cross-tests README](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/pf/tests/internal/cross-tests/README.md).

Examples of cross-tests include:

- [SDKv2 Diff](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/internal/tests/cross-tests/diff_cross_test.go)
- [SDKv2 Create](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/tfbridge/tests/provider_test.go#L405)
- [SDKv2 Configure](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/tfbridge/tests/provider_configure_test.go)
- [PF Configure](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/pf/tests/provider_configure_test.go)
- [PF Diff](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/pf/tests/diff_test.go)


## Property-Based Testing

Building on the cross-tests, we also have property based tests using the [Rapid](https://github.com/flyingmutant/rapid) library. These tests are useful for covering a wide variety of inputs and outputs. These are still somewhat experimental and found under [./pkg/internal/tests/cross-tests/rapid_test.go](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/internal/tests/cross-tests/rapid_test.go#L30).


## GRPC Replay Tests

These are discouraged. New tests should prefer [full schema and program tests](##-integration-tests-using-a-terraform-schema-and-a-pulumi-yaml-program)

The bridge and bridged providers also supports GRPC replay tests. These tests are useful for testing the bridge against a specific GRPC request/response pair. 
Examples can be found under [`./pkg/tfbridge/tests/provider_test.go`](https://github.com/pulumi/pulumi-terraform-bridge/blob/317d4b819f12d4bc66adc5fb248bfb77a3cc7ba7/pkg/tfbridge/provider_test.go#L190). These use the [`providertest.replay` module](https://github.com/pulumi/providertest/tree/main/replay) to replay a specific request/response pair from the Engine <-> Provider GRPC conversation.