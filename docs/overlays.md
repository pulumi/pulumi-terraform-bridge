# Augmenting Auto-Generated Code w/ Overlays (Legacy Feature)

> **Warning** Overlays are a legacy feature and support may be removed in a future version of the bridge.
>             We do not recommend using overlays for a new provider. To augment a terraform provider,
>             please use [tfbridge.ProviderInfo.MuxWith](./muxwith.md).

An overlay is a set of additional (per language files) that the code generator injects when creating the final packages.

These may add additional types, functions, or entire modules in this directory may be merged into the resulting
package.  This can be useful for helper modules and functions, in addition to gradual typing, such as using strongly
typed enums in places where the underlying provider may only have weakly typed strings.

To do this, first add the files in the appropriate package sub-directory of the sdk, and then add the requisite directives to the
provider file.  See the [AWS overlays section in resources.go](https://github.com/pulumi/pulumi-aws/blob/master/provider/resources.go#L4486) for
an example of this in action.
