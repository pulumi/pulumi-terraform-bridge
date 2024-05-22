package crosstests

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func generateYaml(schema shim.SchemaMap, resourceToken string, objectType *tftypes.Object, tfConfig any) (map[string]any, error) {
	pConfig, err := convertConfigToPulumi(schema, nil, objectType, tfConfig)
	if err != nil {
		return nil, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#1864]: schema secrets may be set by convertConfigToPulumi.
	pConfig = propertyvalue.RemoveSecrets(resource.NewObjectProperty(pConfig)).ObjectValue()

	// This is a bit of a leap of faith that serializing PropertyMap to YAML in this way will yield valid Pulumi
	// YAML. This probably needs refinement.
	yamlProperties := pConfig.Mappable()

	data := map[string]any{
		"name":    "project",
		"runtime": "yaml",
		"resources": map[string]any{
			"example": map[string]any{
				"type":       resourceToken,
				"properties": yamlProperties,
			},
		},
		"backend": map[string]any{
			"url": "file://./data",
		},
	}
	return data, nil
}

func convertConfigToPulumi(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	objectType *tftypes.Object,
	tfConfig any,
) (resource.PropertyMap, error) {
	var v *tftypes.Value

	switch tfConfig := tfConfig.(type) {
	case tftypes.Value:
		v = &tfConfig
		if objectType == nil {
			ty := v.Type().(tftypes.Object)
			objectType = &ty
		}
	case *tftypes.Value:
		v = tfConfig
		if objectType == nil {
			ty := v.Type().(tftypes.Object)
			objectType = &ty
		}
	default:
		if objectType == nil {
			t := convert.InferObjectType(schemaMap, nil)
			objectType = &t
		}
		bytes, err := json.Marshal(tfConfig)
		if err != nil {
			return nil, err
		}
		// Knowingly using a deprecated function so we can connect back up to tftypes.Value; if this disappears
		// it should not be prohibitively difficult to rewrite or vendor.
		//
		//nolint:staticcheck
		value, err := tftypes.ValueFromJSON(bytes, *objectType)
		if err != nil {
			return nil, err
		}
		v = &value
	}

	decoder, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   schemaMap,
		SchemaInfos: schemaInfos,
		Object:      objectType,
	})
	if err != nil {
		return nil, err
	}

	// There is not yet a way to opt out of marking schema secrets, so the resulting map might have secrets marked.
	pm, err := convert.DecodePropertyMap(context.Background(), decoder, *v)
	if err != nil {
		return nil, err
	}
	return pm, nil
}
