package tfgen

import (
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
)

func checkIDProperties(sink diag.Sink, spec schema.PackageSpec, info tfbridge.ProviderInfo) error {
	errors := 0

	info.P.ResourcesMap().Range(func(rname string, resource shim.Resource) bool {
		if resourceHasComputeID(info, rname) {
			return true
		}
		ok, reason := resourceHasRegularID(resource)
		if ok {
			return true
		}
		m := fmt.Sprintf("Resource %s has a problem: %s. "+
			"To map this resource consider specifying ResourceInfo.ComputeID",
			rname, reason)
		errors++
		sink.Errorf(&diag.Diag{Message: m})

		return true
	})

	if errors > 0 {
		return fmt.Errorf("There were %d unresolved ID mapping errors", errors)
	}

	return nil
}

func resourceHasRegularID(resource shim.Resource) (bool, string) {
	idSchema, gotID := resource.Schema().GetOk("id")
	if !gotID {
		return false, `no "id" attribute`
	}
	if idSchema.Type() != shim.TypeString {
		return false, `"id" attribute is not of type String`
	}
	if idSchema.Sensitive() {
		return false, `"id" attribute is sensitive`
	}
	return true, ""
}

func resourceHasComputeID(info tfbridge.ProviderInfo, resname string) bool {
	if info.Resources == nil {
		return false
	}
	if info, ok := info.Resources[resname]; ok {
		return info.ComputeID != nil
	}
	return false
}
