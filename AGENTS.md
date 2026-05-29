# AGENTS

This repo is the Pulumi Terraform Bridge. Keep changes small, focused, and consistent with the architecture below.

## What this repo does
- Build-time generation: translate Terraform provider schemas into Pulumi schemas and SDKs.
- Runtime translation: act as a Pulumi provider while driving a Terraform provider under the hood.
- These two pipelines are tightly coupled; changes in one can affect the other.

## Architecture map
- Build-time generation: `pkg/tfgen`, `pkg/convert`, `pkg/tf2pulumi`
- Runtime bridge for Terraform Plugin SDK v2: `pkg/tfbridge`, `pkg/tfshim/sdk-v{1,2}`, `pkg/providerserver`
- Runtime bridge for Terraform Plugin Framework: `pkg/pf`, `pkg/tfshim/schema`
- Muxed providers: `pkg/x/muxer`
- Dynamic bridge: `dynamic/*`
- Experimental APIs: `unstable/*`
- Tests: `pkg/tests`, `pkg/internal/tests/cross-tests`, `pkg/pf/tests`, `internal/testing`
- Ops and tooling: `Makefile`, `scripts/`, `tools/`

The project has multiple Go module roots, including the repo root and `tools/`.

## Toolchain and workspace
- Use tool versions from `mise.toml` when available.
- Run `make go.work` when local Go commands need the nested module workspace initialized or refreshed.
- Do not hand-edit generated SDKs, generated schema artifacts, vendored upstream provider code, or submodule content.

## Workflow guidance
- Identify whether a change is build-time, runtime, or both. Update tests that cover the affected path.
- Favor local changes near the relevant layer; avoid cross-layer coupling unless required.
- Preserve existing diff, plan, state, import, and refresh behavior unless explicitly changing semantics.
- If adding runtime behavior, confirm it works for both SDKv2 and PF paths, or document why it is specific to one path.
- For muxed-provider behavior, check the muxing layer explicitly instead of assuming SDKv2 or PF behavior applies unchanged.
- Treat golden-file updates as behavioral evidence. Use acceptance mode only when the expected output intentionally changes.
- For non-trivial bridge behavior or spec work, use `.agents/skills/create-bridge-spec/SKILL.md` to ground the plan in behavior, path, ownership, and proof.

## Useful entry points
- Build-time schema, docs, and generation: `pkg/tfgen`, `pkg/convert`, `pkg/tf2pulumi`
- Runtime provider (SDKv2): `pkg/tfbridge`, `pkg/tfshim/sdk-v2`, `pkg/providerserver`
- Runtime provider (PF): `pkg/pf/tfbridge`, `pkg/pf/internal`, `pkg/tfshim/schema`
- Muxing and provider composition: `pkg/x/muxer`
- Cross-cutting state, defaults, secrets, and logging: `pkg/tfbridge`, `pkg/convert`, `internal/logging`
- Deeper architecture map: `docs/architecture/overview.md`

## Tests
- Prefer targeted tests in the existing harness for the mode you touch.
- Common locations: `pkg/tests`, `pkg/internal/tests/cross-tests`, `pkg/pf/tests`, and `internal/testing`.
- Use `make test RUN_TEST_CMD='./path/to/package -run TestName'` for targeted repo tests so plugin setup, the test provider binary, coverage flags, and timeouts match the repo harness.
- Raw `go test` is fine only for simple package/unit cases that do not depend on bridge test setup.
- Use cross-tests in `pkg/internal/tests/cross-tests` or `pkg/pf/tests/internal/cross-tests` when validating Pulumi/Terraform parity.
- To update golden outputs intentionally, use `make test_accept` and review the resulting diff carefully.
- Testing guide: `docs/guides/testing.md`

## Commands
- Build: `make build`
- Initialize/update Go workspace: `make go.work`
- Install test plugins: `make install_plugins`
- Full test suite: `make test`
- Accept/update golden test output: `make test_accept`
- Lint: `make lint`
- Lint with fixes: `make lint_fix`
- Format Go code: `make fmt`
- Tidy all Go modules: `make tidy`
- Generate built-in tests: `make generate_builtins_test`
- Scripted lint entrypoint: `go run scripts/build.go lint`
- Scripted lint fix entrypoint: `go run scripts/build.go fix-lint`

## Environment variables
These variables affect `tfgen` and conversion behavior:

- `PULUMI_SKIP_MISSING_MAPPING_ERROR`: skip errors for unmapped resources.
- `PULUMI_SKIP_EXTRA_MAPPING_ERROR`: skip errors for extra mappings.
- `PULUMI_MISSING_DOCS_ERROR`: fail on missing documentation.
- `PULUMI_CONVERT`: HCL to PCL conversion is enabled by default; set `PULUMI_CONVERT=0` to disable it.
- `PULUMI_CONVERT_ONLY`: convert docs for a single resource or data source while debugging docs issues.
- `PULUMI_CONVERT_AUTOFILL_DIR`: configure example autofill data.
- `COVERAGE_OUTPUT_DIR`: generate conversion coverage reports.

## Docs
- Architecture overview: `docs/architecture/overview.md`
- Guides: `docs/guides/*`
