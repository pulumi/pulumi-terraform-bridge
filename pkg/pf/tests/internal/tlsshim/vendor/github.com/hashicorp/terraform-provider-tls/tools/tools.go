//go:build tools

package tools

// Including Terraform documentation generator following the
// ["tools as dependencies"](https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md)
// model.

import (
	// [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs).
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
