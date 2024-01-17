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
		_, gotID := resource.Schema().GetOk("id")
		if gotID {
			return true
		}

		if info.Resources != nil {
			if info, ok := info.Resources[rname]; ok {
				if info.ComputeID != nil {
					return true
				}
			}
		}

		m := fmt.Sprintf("Resource %s does not have id property defined. "+
			"To map this resource properly consider specifying ResourceInfo.ComputeID",
			rname)
		errors++
		sink.Errorf(&diag.Diag{Message: m})

		return true
	})

	if errors > 0 {
		return fmt.Errorf("There were %d unresolved ID mapping errors", errors)
	}

	return nil
}
