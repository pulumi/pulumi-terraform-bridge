package internal

import (
	"os"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func getDefaultValue(d *tfbridge.DefaultInfo, res *tfbridge.PulumiResource) (interface{}, error) {
	if d == nil {
		return nil, nil
	}
	if d.From != nil {
		return d.From(res)
	} else if d.EnvVars != nil {
		for _, n := range d.EnvVars {
			if v, ok := os.LookupEnv(n); ok {
				return v, nil
			}
		}
	}
	return d.Value, nil
}

func SetDefaultValues(res *tfbridge.PulumiResource, fields map[string]*tfbridge.SchemaInfo) error {
	for key, fld := range fields {
		if _, ok := res.Properties[resource.PropertyKey(key)]; !ok && fld.Default != nil {
			// using default value for empty property
			v, err := getDefaultValue(fld.Default, res)
			if err != nil {
				return err
			}
			if pval, ok := v.(resource.PropertyValue); ok {
				res.Properties[resource.PropertyKey(key)] = pval
			}
			res.Properties[resource.PropertyKey(key)] = resource.NewPropertyValue(v)
		}
	}

	return nil
}
