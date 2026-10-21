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
- Golden output depends on which Pulumi plugins are installed, not just on the code. Regenerate goldens only against the plugin set CI uses:
  - Run `mise install` first. Its postinstall hook clears only `.pulumi/plugins` and links the external plugin pins from `mise.toml` into the expected Pulumi cache layout. Automatic Pulumi plugin acquisition is disabled. Normal tests and `make test_accept` use that same repo-local cache.
  - Finish setup before starting tests. Do not run `mise install` or `mise run sync-plugins` while tests in that checkout are running. Separate worktrees have separate plugin directories.
  - For direct Go commands, including in `dynamic/`, use `mise exec -- go test ...` after setup.
  - Use `make test_plugin_isolation` to check clean and contaminated environments, concurrent acceptance, and repeated golden generation in disposable worktrees.
  - `PULUMI_ACCEPT=1` only refreshes the repo's own golden helpers, such as `TestSchemaGenerationFullDocs`. `autogold` tests, such as `TestPlainDocsParser`, also need `-update`.
  - Unset `PULUMI_MISSING_DOCS_ERROR` before running tests. Several `pkg/tfgen` tests fail when it's set.
  - If a regenerated golden changes something unrelated to your change, suspect the plugin set before accepting it.
- Testing guide: `docs/guides/testing.md`

## Commands
- Build: `make build`
- Initialize/update Go workspace: `make go.work`
- Install tools and test plugins: `mise install`
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
