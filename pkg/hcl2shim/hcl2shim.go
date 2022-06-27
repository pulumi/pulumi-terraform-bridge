package hcl2shim

import (
	"github.com/hashicorp/go-cty/cty"
	tfHcl2shim "github.com/hashicorp/terraform-plugin-sdk/v2/internal/configs/hcl2shim"
)

func HCL2ValueFromConfigValue(v interface{}) cty.Value {
	return tfHcl2shim.HCL2ValueFromConfigValue(v)
}
