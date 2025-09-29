# Testing the Pulumi Terraform Bridge

The bridge relies on several complementary test layers to make sure Pulumi behavior stays aligned with Terraform while
honoring Pulumi semantics. This guide explains when to reach for each layer, how to run it locally, and where to look for
examples.

## Quick Reference

### Recommended suites

| Use case | Recommended suite | Location | Notes |
| -------- | ----------------- | -------- | ----- |
| Unit logic (helpers, conversions) | Go unit tests | Same package (`*_test.go`) | Fast feedback without Pulumi/Terraform harnesses. |
| Runtime behavior with Pulumi engine (SDKv2) | Schema + program integration tests | `pkg/tests/` | Runs Pulumi programs via Automation API to exercise full engine↔provider flows; default choice for bridge work. |
| Runtime behavior with Pulumi engine (PF) | PF schema + program tests | `pkg/pf/tests/` | Mirrors SDKv2 harness with PF builders and Pulumi Automation. |
| Terraform parity | Cross-tests | `pkg/internal/tests/cross-tests/`, `pkg/pf/tests/internal/cross-tests/` | Compares Terraform CLI vs Pulumi for the same provider. |
| Property fuzzing | Rapid-based cross-tests | `pkg/internal/tests/cross-tests/rapid_test.go` | Generates many input combos; slower but valuable for tricky schemas. |

### Special-case / legacy suites

| Use case | Suite | Location | Notes |
| -------- | ----- | -------- | ----- |
| Provider-only runtime behavior (SDKv2) | Provider server tests | `pkg/tfbridge/tests/` | Calls the bridge directly without Pulumi; use only when Automation-backed tests cannot cover the scenario. |
| Repro recorded RPCs | Replay tests | `pkg/tfbridge/provider_test.go` | Legacy gRPC recordings; prefer higher-level harnesses whenever possible. |

## Local Test Commands

```bash
# Fast lint + unit
make lint
make test RUN_TEST_CMD=./pkg/tfbridge -run TestSomeUnit

# SDKv2 integration (single file)
make test RUN_TEST_CMD='./pkg/tests -run TestFoo'

# PF integration
make test RUN_TEST_CMD='./pkg/pf/tests -run TestBar'

# Cross-tests (SDKv2)
make test RUN_TEST_CMD=./pkg/internal/tests/cross-tests

# Accept golden updates after intentional diff changes
make test_accept
```

`make test` installs required Pulumi plugins and builds the test provider binary, so the first run can take several minutes.

## Choosing the Right Layer

1. **Start simple** – If logic can be isolated, write a pure Go unit test.
2. **Touching resource CRUD, diffing, or provider config** – use schema + program tests to exercise Pulumi RPC flows.
3. **Parity regression** – Write or update a cross-test so Terraform CLI and Pulumi remain aligned.
4. **Protocol edge cases** – If reproducing a live gRPC sequence is easiest, add a replay test but translate it into a higher-level test when possible.

## Schema + Program Integration Tests

Integration tests spin up a minimal Terraform provider schema and drive it through a Pulumi program using
[`pulumiTest`](https://github.com/pulumi/providertest/tree/main/pulumitest). They are the preferred way to cover
end-to-end scenarios, including multi-step workflows.

### SDKv2 Example

`pkg/tests/schema_pulumi_test.go` shows the basic pattern:

```go
func TestBasic(t *testing.T) {
    t.Parallel()

    tfResourceMap := map[string]*schema.Resource{
        "prov_test": {
            Schema: map[string]*schema.Schema{
                "test": {Type: schema.TypeString, Optional: true},
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

Key points:

- Build the minimal Terraform schema necessary to exercise the behavior under test.
- Wrap the Terraform provider with `pulcheck.BridgedProvider` to obtain a Pulumi provider.
- Use a short Pulumi program (YAML works well) and assert on outputs, state transitions, or logs through the
  `pulumiTest` harness.

### Plugin Framework Example

PF tests follow the same pattern with different helpers (`pkg/pf/tests/schema_and_program_test.go`):

```go
func TestBasic(t *testing.T) {
    t.Parallel()

    provBuilder := providerbuilder.NewProvider(providerbuilder.NewProviderArgs{
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
  testOut: ${mainRes.s}
`

    pt, err := pulcheck.PulCheck(t, prov, program)
    require.NoError(t, err)

    upResult := pt.Up(t)
    require.Equal(t, "hello", upResult.Outputs["testOut"].Value)
}
```

PF adds builders to compose provider resources and separate `pulcheck` helpers, but the assertions and workflow match the
SDKv2 pattern.

When you need to validate the full Pulumi engine ↔ provider interaction, prefer these Pulumi-backed suites (`pkg/tests/`
for Plugin SDK and `pkg/pf/tests/` for Plugin Framework). They surface Automation API behavior, secrets handling, and
multi-step workflows exactly as users experience them and should be the default choice for new coverage.

## Provider-Only Runtime Tests

`pkg/tfbridge/tests/` exercises the bridge without running a Pulumi program. Tests construct a provider server directly
via helpers such as `newTestProvider` or `crosstests.MakeConfigure`, then call RPCs (or feed gRPC recordings) to inspect
the raw bridge behavior. See `pkg/tfbridge/tests/provider_configure_test.go` for configure parity coverage and
`pkg/tfbridge/tests/provider_test.go` for gRPC replay and regression scenarios.

> If you are not certain you need a provider-only test, you almost certainly want a Pulumi-backed test in `pkg/tests/`
> instead.

Use this suite when:

- You have a specific reason to bypass Automation API.
- You maintain paired coverage: a Pulumi-backed test in `pkg/tests/` plus a provider-only variant in
  `pkg/tfbridge/tests/` to isolate whether a regression requires Pulumi in the loop.

Provider-only tests complement, not replace, Pulumi-backed coverage: once behavior is stable, ensure user-visible flows
still pass under `pkg/tests/` so Pulumi integration remains exercised end-to-end.

## Cross-Tests

Cross-tests run Terraform CLI and the Pulumi bridge against the same inputs, then compare results. They live under
`pkg/internal/tests/cross-tests/` (SDKv2) and `pkg/pf/tests/internal/cross-tests/` (PF). Use them when validating parity
or investigating production regressions.

Examples worth consulting:

- Diff parity (`pkg/internal/tests/cross-tests/diff_cross_test.go`)
- Create/Update flows (`pkg/tfbridge/tests/provider_test.go`)
- Provider configuration (`pkg/tfbridge/tests/provider_configure_test.go`, `pkg/pf/tests/provider_configure_test.go`)
- PF-specific diffing (`pkg/pf/tests/diff_test.go`)

## Property-Based Tests

Property-based tests extend cross-tests with randomized inputs using the
[Rapid](https://github.com/flyingmutant/rapid) library (`pkg/internal/tests/cross-tests/rapid_test.go`). Because they
explore many scenarios, they take longer to run and can be noisy; prefer them when chasing subtle schema interactions.

## gRPC Replay Tests (Legacy)

Replay tests feed recorded Pulumi Engine ↔ Provider RPC traces back through the bridge (see
`pkg/tfbridge/provider_test.go`). They are helpful when higher-level harnesses are impractical, but new coverage should
use schema + program or cross-tests instead. Treat replays as stop-gaps and backfill more maintainable coverage later.

## Golden Files & Accepting Changes

Some tests assert against generated output stored alongside fixtures. When behavior changes intentionally:

1. Run `make test_accept` (sets `PULUMI_ACCEPT=1`) to update goldens.
2. Inspect diffs carefully and justify them in your PR description.
3. Coordinate with maintainers when changes could affect downstream providers.

## Debugging Failures

- Set `PULUMI_DEBUG_GRPC=1` for verbose Pulumi provider logs.
- Enable Terraform tracing with `TF_LOG=DEBUG` or `TF_LOG=TRACE` when Terraform behavior is surprising.

## Continuous Integration

- GitHub Actions run `make lint` and `make test` with caching.
- Coverage reports aggregate into `coverage.txt` and feed the configuration in `codecov.yml`.
