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
	Custom  bool
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

func Convert(inputFile string, outputDir string) (StackExport, error) {
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

func getProviderInputs(providerName string) (resource.PropertyMap, error) {
	// TODO: call the CheckConfig GRPC method
	if providerName != "aws" {
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
	return resource.PropertyMap{
		"region":                    resource.NewProperty("us-east-1"),
		"skipCredentialsValidation": resource.NewProperty(false),
		"skipRegionValidation":      resource.NewProperty(true),
		"version":                   resource.NewProperty("7.11.1"),
	}, nil
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

func convertState(tfState *TerraformState, pulumiProviders map[string]*info.Provider) (*PulumiState, error) {
	pulumiState := &PulumiState{}

	inputs, err := getProviderInputs("aws")
	if err != nil {
		return nil, fmt.Errorf("failed to get provider inputs: %w", err)
	}
	// add a provider resources
	prov := PulumiResource{
		Custom:  true,
		ID:      "a339fe8e-e15d-4203-8719-c0ca5d3f414e", // TODO: This is wrong, how is it generated?
		Type:    "pulumi:providers:aws",
		Name:    "default_7_11_1",
		Inputs:  inputs,
		Outputs: inputs,
	}
	pulumiState.Providers = append(pulumiState.Providers, prov)
	for _, resource := range tfState.Resources {
		// TODO: match the provider URN to the resource
		prov, ok := pulumiProviders[resource.ProviderName]
		if !ok {
			return nil, fmt.Errorf("no Pulumi provider found for Terraform provider: %s", resource.ProviderName)
		}
		shimResource := prov.P.ResourcesMap().Get(resource.TypeName)
		if shimResource == nil {
			return nil, fmt.Errorf("no resource type found for Terraform resource: %s", resource.TypeName)
		}

		valueshimResourceType := shimResource.SchemaType()

		ctyValue, err := resourceToCtyValue(&resource, valueshimResourceType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert resource to CTY value: %w", err)
		}

		resourceInfo := prov.Resources[resource.TypeName]
		pulumiTypeToken := resourceInfo.Tok
		if pulumiTypeToken == "" {
			camelName, pascalName := camelPascalPulumiName(resource.TypeName, prov)
			pkgName := tokens.NewPackageToken(tokens.PackageName(tokens.IntoQName(prov.Name)))
			modTok := tokens.NewModuleToken(pkgName, tokens.ModuleName(camelName))
			pulumiTypeToken = tokens.NewTypeToken(modTok, tokens.TypeName(pascalName))
		}
		props, err := convertTFValueToPulumiValue(ctyValue, resource.TypeName, shimResource.Schema(), resourceInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to Pulumi value: %w", err)
		}
		pulumiState.Resources = append(pulumiState.Resources, PulumiResource{
			Custom:  true,
			ID:      props["id"].StringValue(),
			Type:    string(pulumiTypeToken),
			Inputs:  props,
			Outputs: props,
			Name:    resource.Name,
			// Parent:   stackUrn,
			// Provider: providerUrn,
		})
	}

	return pulumiState, nil
}
