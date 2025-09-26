# pkg/tfbridge

`pkg/tfbridge` hosts the runtime entry point that exposes a Terraform provider as a Pulumi resource provider. Key
responsibilities include:

- Implementing Pulumi RPC methods (`Create`, `Read`, `Update`, `Delete`, `Invoke`, `Plan`, `CheckConfig`, etc.).
- Translating Pulumi `PropertyValue`s to Terraform schemas and state via `pkg/tfshim` shims.
- Coordinating ProviderInfo overlays (`pkg/tfbridge/info`) that customize resource behavior, docs, and metadata.
- Managing diff semantics, replacements, and detailed diff reporting.
- Handling secrets, defaults, and provider configuration.

See `docs/architecture/overview.md` for a high-level map and `docs/guides/testing.md` for recommended tests when
touching this package.
