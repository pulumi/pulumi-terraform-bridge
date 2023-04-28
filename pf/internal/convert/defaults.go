package convert

import (
	"os"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func GetDefaultValue(d *tfbridge.DefaultInfo, resource *tfbridge.PulumiResource) (interface{}, error) {
	if d == nil {
		return nil, nil
	}
	if d.From != nil {
		return d.From(resource)
	} else if d.EnvVars != nil {
		for _, n := range d.EnvVars {
			if v := os.Getenv(n); v != "" {
				return v, nil
			}
		}
	}
	return d.Value, nil
}
