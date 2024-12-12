# Pulumi Bridge for Terraform Plugin Framework

This bridge enables creating [Pulumi Resource providers](https://www.pulumi.com/docs/intro/concepts/resources/providers/) from [Terraform Providers](https://github.com/terraform-providers)
built using the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework).

If you need to adapt [Terraform Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) based providers, see [the documentation for
bridging a new SDK based provider](../../docs/guides/new-provider.md).

If you have a Pulumi provider that was bridged from a Terraform provider built against
[Terraform Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) and you want to upgrade it to a version that has migrated some
but not all resources/datasources to the [Plugin Framework](https://github.com/hashicorp/terraform-plugin-sdk?tab=readme-ov-file), see [here](../../docs/guides/upgrade-sdk-to-mux.md).

## Docs

- If you are interested in bridging a new provider, see [Developing a New Plugin Framework Provider](../../docs/guides/new-pf-provider.md).
- If you are interested in upgrading a bridge provider to use PF, see [Upgrading a Bridged Provider to Plugin Framework](../../docs/guides/upgrade-sdk-to-pf.md).
