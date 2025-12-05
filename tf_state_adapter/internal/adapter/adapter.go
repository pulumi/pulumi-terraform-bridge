package adapter

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type PulumiResource struct {
	ID      string
	Name    string
	Type    string
	Inputs  resource.PropertyMap
	Outputs resource.PropertyMap
	// Parent   string
	// Provider string
}

type PulumiState struct {
	Resources []PulumiResource
	Providers []PulumiResource
}

type StackExport struct {
	Deployment apitype.DeploymentV3 `json:"deployment"`
	Version    int                  `json:"version"`
}

type ResourceExport struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Inputs  map[string]any `json:"inputs"`
	Outputs map[string]any `json:"outputs"`
}

func ConvertState(inputFile string, outputDir string) (StackExport, error) {
	tfState, err := readTerraformState(inputFile)
	if err != nil {
		return StackExport{}, fmt.Errorf("failed to read Terraform state: %w", err)
	}

	pulumiProviders, err := getProviders(tfState)
	if err != nil {
		return StackExport{}, fmt.Errorf("failed to get resource types: %w", err)
	}

	pulumiState, err := convertState(tfState, pulumiProviders)
	if err != nil {
		return StackExport{}, fmt.Errorf("failed to convert state: %w", err)
	}

	data, err := MakeDeployment(pulumiState, outputDir)
	if err != nil {
		return StackExport{}, fmt.Errorf("failed to write Pulumi state: %w", err)
	}

	return StackExport{
		Deployment: data,
		Version:    3,
	}, nil
}

func findResource(tfState *TerraformState, resourceAddress string) (TerraformResource, error) {
	for _, resource := range tfState.Resources {
		if resource.Address == resourceAddress {
			return resource, nil
		}
	}
	return TerraformResource{}, fmt.Errorf("resource not found: %s", resourceAddress)
}

func ConvertResourceState(inputFile string, resourceAddress string, outputFile string) (ResourceExport, error) {
	tfState, err := readTerraformState(inputFile)
	if err != nil {
		return ResourceExport{}, fmt.Errorf("failed to read Terraform state: %w", err)
	}

	pulumiProviders, err := getProviders(tfState)
	if err != nil {
		return ResourceExport{}, fmt.Errorf("failed to get resource types: %w", err)
	}

	res, err := findResource(tfState, resourceAddress)
	if err != nil {
		return ResourceExport{}, fmt.Errorf("failed to find resource: %w", err)
	}

	resourceState, err := convertResourceState(res, pulumiProviders)
	if err != nil {
		return ResourceExport{}, fmt.Errorf("failed to convert resource state: %w", err)
	}

	return ResourceExport{
		ID:      resourceState.ID,
		Name:    resourceState.Name,
		Type:    resourceState.Type,
		Inputs:  resourceState.Inputs.Mappable(),
		Outputs: resourceState.Outputs.Mappable(),
	}, nil
}

func getProviders(tfState *TerraformState) (map[string]*info.Provider, error) {
	tfProviders := tfState.Providers
	pulumiProviders := make(map[string]*info.Provider, len(tfProviders))
	// TODO: mapping from Terraform provider names to Pulumi provider names
	mapping := map[string]string{
		"registry.terraform.io/hashicorp/archive": "archive",
		"registry.opentofu.org/hashicorp/archive": "archive",
		"registry.terraform.io/hashicorp/aws":     "aws",
		"registry.opentofu.org/hashicorp/aws":     "aws",
		"registry.terraform.io/hashicorp/random":  "random",
		"registry.opentofu.org/hashicorp/random":  "random",
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

func getProviderInputs(providerName string) (resource.PropertyMap, error) {
	// TODO: call the CheckConfig GRPC method
	switch providerName {
	case "aws":
		return resource.PropertyMap{
			"region":                    resource.NewProperty("us-east-1"),
			"skipCredentialsValidation": resource.NewProperty(false),
			"skipRegionValidation":      resource.NewProperty(true),
			"version":                   resource.NewProperty("7.11.1"),
		}, nil
	case "archive":
		return resource.PropertyMap{
			"version": resource.NewProperty("0.3.5"),
		}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", providerName)
}

// copied from pkg/tfbridge/provider.go
// TODO: share this
func camelPascalPulumiName(name string, prov *info.Provider) (string, string) {
	prefix := prov.GetResourcePrefix() + "_"
	contract.Assertf(strings.HasPrefix(name, prefix),
		"Expected all Terraform resources in this module to have a '%v' prefix (%q)", prefix, name)
	name = name[len(prefix):]
	camel := tfbridge.TerraformToPulumiNameV2(name, nil, nil)
	pascal := camel
	if pascal != "" {
		pascal = string(unicode.ToUpper(rune(pascal[0]))) + pascal[1:]
	}
	return camel, pascal
}

func pulumiTypeToken(tfTypeName string, pulumiProvider *info.Provider) (tokens.Type, error) {
	resourceInfo := pulumiProvider.Resources[tfTypeName]
	if resourceInfo.Tok != "" {
		return resourceInfo.Tok, nil
	}
	camelName, pascalName := camelPascalPulumiName(tfTypeName, pulumiProvider)
	pkgName := tokens.NewPackageToken(tokens.PackageName(tokens.IntoQName(pulumiProvider.Name)))
	modTok := tokens.NewModuleToken(pkgName, tokens.ModuleName(camelName))
	return tokens.NewTypeToken(modTok, tokens.TypeName(pascalName)), nil
}

func convertState(tfState *TerraformState, pulumiProviders map[string]*info.Provider) (*PulumiState, error) {
	pulumiState := &PulumiState{}

	for _, provider := range pulumiProviders {
		inputs, err := getProviderInputs(provider.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider inputs: %w", err)
		}
		pulumiState.Providers = append(pulumiState.Providers, PulumiResource{
			ID:      "a339fe8e-e15d-4203-8719-c0ca5d3f414e", // TODO: This is wrong, how is it generated?
			Type:    "pulumi:providers:" + provider.Name,
			Name:    "default_" + strings.ReplaceAll(provider.Version, ".", "_"),
			Inputs:  inputs,
			Outputs: inputs,
		})
	}
	for _, resource := range tfState.Resources {
		if resource.Mode == "data" {
			continue
		}
		pulumiResource, err := convertResourceState(resource, pulumiProviders)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resource state: %w", err)
		}
		pulumiState.Resources = append(pulumiState.Resources, pulumiResource)
	}

	return pulumiState, nil
}

func convertResourceState(res TerraformResource, pulumiProviders map[string]*info.Provider) (PulumiResource, error) {
	// TODO: match the provider URN to the resource
	prov, ok := pulumiProviders[res.ProviderName]
	if !ok {
		return PulumiResource{}, fmt.Errorf("no Pulumi provider found for Terraform provider: %s", res.ProviderName)
	}
	shimResource := prov.P.ResourcesMap().Get(res.TypeName)
	if shimResource == nil {
		return PulumiResource{}, fmt.Errorf("no resource type found for Terraform resource: %s", res.TypeName)
	}

	valueshimResourceType := shimResource.SchemaType()

	ctyValue, err := resourceToCtyValue(&res, valueshimResourceType)
	if err != nil {
		return PulumiResource{}, fmt.Errorf("failed to convert resource to CTY value: %w", err)
	}

	pulumiTypeToken, err := pulumiTypeToken(res.TypeName, prov)
	if err != nil {
		return PulumiResource{}, fmt.Errorf("failed to get Pulumi type token: %w", err)
	}
	resourceInfo := prov.Resources[res.TypeName]
	props, err := convertTFValueToPulumiValue(ctyValue, res.TypeName, shimResource.Schema(), resourceInfo)
	if err != nil {
		return PulumiResource{}, fmt.Errorf("failed to convert value to Pulumi value: %w", err)
	}

	inputs, err := tfbridge.ExtractInputsFromOutputs(resource.PropertyMap{}, props, shimResource.Schema(), resourceInfo.Fields, false)
	if err != nil {
		return PulumiResource{}, fmt.Errorf("failed to extract inputs from outputs: %w", err)
	}

	return PulumiResource{
		ID:      props["id"].StringValue(),
		Type:    string(pulumiTypeToken),
		Inputs:  inputs,
		Outputs: props,
		Name:    res.Name,
		// Parent:   stackUrn,
		// Provider: providerUrn,
	}, nil
}
