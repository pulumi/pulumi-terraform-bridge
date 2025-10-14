# pkg/tfshim

`pkg/tfshim` normalizes Terraform provider APIs so the bridge can interact with multiple Terraform runtimes. It provides
interfaces that abstract over:

- **Plugin SDK v1/v2** (`pkg/tfshim/sdk-v1`, `pkg/tfshim/sdk-v2`) – exposes provider, resource, diff, and state helpers.
- **Schema translation** (`pkg/tfshim/schema`) – shared types used by the Plugin Framework bridge.
- **Utilities** (`pkg/tfshim/util`, `pkg/tfshim/diagnostics`) – helpers for working with Terraform diagnostics, types,
  and schema walking.

The goal is to keep Terraform-specific details contained here so `pkg/tfbridge` and `pkg/pf/tfbridge` can be written in
terms of consistent interfaces.

When adding new Terraform APIs, extend the shim interfaces first, then adapt individual provider implementations. Tests
live alongside each versioned shim.
