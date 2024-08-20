# pulumi-hcl-lint

Lightweight linter for [HCL](https://github.com/hashicorp/hcl) files.

Detects undeclared resource references.

Not critical for bridging providers. This tool is used to assist documentation generation and example cleanup.

## Usage

Run in a folder with one or more `.tf` files:

    ./pulumi-hcl-lint

Command-line options:

```shell
Usage of ./pulumi-hcl-lint:
  -json
        Emit output in JSON format
  -out string
        Emit output to the given file
```
