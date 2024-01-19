# `tfbridge.ProviderInfo.MuxWith`

[`tfbridge.ProviderInfo.MuxWith`](https://github.com/pulumi/pulumi-terraform-bridge/blob/5e17c6c7e2d877db7e1d9c0b953a06d3ecabbaea/pkg/tfbridge/info.go#L126-L133)
allows the mixin (muxing) of other providers to the bridged upstream Terraform
provider. With a provider mixin it's possible to add or replace resources and/or functions
(data sources) in the wrapped Terraform provider without having to change the upstream
code itself.
