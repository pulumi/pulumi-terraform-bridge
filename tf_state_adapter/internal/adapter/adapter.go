package adapter

import (
	"encoding/json"
	"fmt"
	"q"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type PulumiState struct {
	Resources map[string]resource.PropertyMap `json:"resources"`
}

func Convert(inputFile string) ([]byte, error) {
	tfState, err := readTerraformState(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read Terraform state: %w", err)
	}

	pulumiProviders, err := getProviders(tfState)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource types: %w", err)
	}

	pulumiState, err := convertState(tfState, pulumiProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to convert state: %w", err)
	}

	data, err := writePulumiState(pulumiState)
	if err != nil {
		return nil, fmt.Errorf("failed to write Pulumi state: %w", err)
	}

	return data, nil
}

func getProviders(tfState *TerraformState) (map[string]*info.Provider, error) {
	tfProviders := tfState.Providers
	pulumiProviders := make(map[string]*info.Provider, len(tfProviders))
	// TODO: mapping from Terraform provider names to Pulumi provider names
	mapping := map[string]string{
		"registry.terraform.io/hashicorp/archive": "archive",
		"registry.terraform.io/hashicorp/aws":     "aws",
		"registry.opentofu.org/hashicorp/aws":     "aws",
		"registry.terraform.io/hashicorp/random":  "random",
	}
	for _, provider := range tfProviders {
		pulumiName := mapping[provider]
		if pulumiName == "" {
			return nil, fmt.Errorf("no Pulumi provider name found for Terraform provider: %s", provider)
		}
		prov, err := getProviderSchema(pulumiName)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider schema: %w", err)
		}
		pulumiProviders[provider] = prov
	}
	return pulumiProviders, nil
}

func convertState(tfState *TerraformState, pulumiProviders map[string]*info.Provider) (*PulumiState, error) {
	pulumiState := &PulumiState{
		Resources: make(map[string]resource.PropertyMap),
	}
	for _, resource := range tfState.Resources {
		prov, ok := pulumiProviders[resource.ProviderName]
		if !ok {
			return nil, fmt.Errorf("no Pulumi provider found for Terraform provider: %s", resource.ProviderName)
		}
		shimResource := prov.P.ResourcesMap().Get(resource.TypeName)
		q.Q(shimResource)
		if shimResource == nil {
			return nil, fmt.Errorf("no resource type found for Terraform resource: %s", resource.TypeName)
		}

		valueshimResourceType := shimResource.SchemaType()
		q.Q(valueshimResourceType)

		ctyValue, err := resourceToCtyValue(&resource, valueshimResourceType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resource to CTY value: %w", err)
		}

		pulumiResourceName := resource.TypeName
		props, err := convertTFValueToPulumiValue(ctyValue, resource.TypeName, shimResource.Schema(), prov.Resources[pulumiResourceName])
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to Pulumi value: %w", err)
		}
		pulumiState.Resources[resource.Name] = props
	}

	return pulumiState, nil
}

func writePulumiState(state *PulumiState) ([]byte, error) {
	jsonState := map[string]interface{}{}
	for name, props := range state.Resources {
		jsonState[name] = props.Mappable()
	}
	data, err := json.MarshalIndent(jsonState, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}
