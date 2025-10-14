# pkg/convert

`pkg/convert` translates between Terraform values (cty) and Pulumi `resource.PropertyValue`s. The bridge uses it during
schema generation, diffing, and runtime conversions to ensure types and secrets propagate correctly.

Highlights:

- Type-specific converters (`bool.go`, `number.go`, `object.go`, etc.) map Terraform primitives and collections.
- `adapter.go` exposes the entry points consumed by `pkg/tfbridge` and `pkg/tfgen`.
- Secret handling (`secret.go`) and unknown values are treated carefully to preserve Pulumi semantics.

Touch this package when Terraform adds new value shapes or when Pulumi gains richer type modeling. Add unit tests next to
the converter you modify.
