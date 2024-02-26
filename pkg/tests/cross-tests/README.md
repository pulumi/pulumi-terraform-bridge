# cross-tests

Tests to compare and contrast the behavior of a given Terraform provider under Terraform CLI against
the behavior of the same provider bridged to Pulumi and used under Pulumi CLI.

To make debugging easier and avoid an extra build step, these tests start both Pulumi and Terraform
providers in-process and have the CLIs attach to these in-process providers.

Custom dependencies are needed temporarily before the appropriate changes land in pulumi/pulumi and
pulumi/pulumi-yaml. `nix develop` will get a shell with all custom dependencies installed.
Alternatively, if the editor is integrated with `direnv`, it will run `nix develop` automatically
when entering this directory.
