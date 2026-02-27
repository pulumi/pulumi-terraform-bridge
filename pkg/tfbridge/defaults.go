package tfbridge

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

// deleteDefaultsKey removes empty `__defaults: []` entries from all objects recursively.
func deleteDefaultsKey(inputs resource.PropertyMap) {
	if defaults, ok := inputs[reservedkeys.Defaults]; ok && isEmptyDefaults(defaults) {
		delete(inputs, reservedkeys.Defaults)
	}
	for key, value := range inputs {
		newVal, err := propertyvalue.TransformPropertyValue(
			resource.PropertyPath{},
			func(_ resource.PropertyPath, pv resource.PropertyValue) (resource.PropertyValue, error) {
				if pv.IsObject() {
					obj := pv.ObjectValue()
					if defaults, ok := obj[reservedkeys.Defaults]; ok && isEmptyDefaults(defaults) {
						delete(obj, reservedkeys.Defaults)
					}
				}
				return pv, nil
			},
			value,
		)
		if err == nil {
			inputs[key] = newVal
		}
	}
}

func isEmptyDefaults(v resource.PropertyValue) bool {
	return v.IsArray() && len(v.ArrayValue()) == 0
}
