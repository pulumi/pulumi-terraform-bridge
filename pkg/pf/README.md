# pkg/pf

`pkg/pf` contains the bridge implementation for Terraform Plugin Framework (PF) providers.

- `tfbridge/` mirrors the SDKv2 bridge logic for PF (diffing, CRUD, config, invoke).
- `internal/` and `proto/` host shared PF-specific utilities and generated protocol definitions.
- `tests/` contains PF-oriented schema+program and cross-tests.

Use this package when bringing up new PF-based providers or migrating existing bridged providers resource-by-resource
with muxing support (`pkg/x/muxer`). Refer to `docs/guides/upgrade-sdk-to-pf.md` and `docs/architecture/overview.md` for
context.
