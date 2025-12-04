package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// TODO: Dependencies
// TODO: Sensitive values
type TerraformResource struct {
	ProviderName  string                 `json:"provider_name"`
	SchemaVersion int                    `json:"schema_version"`
	TypeName      string                 `json:"type"`
	Name          string                 `json:"name"`
	Address       string                 `json:"address"`
	Mode          string                 `json:"mode"`
	Values        map[string]interface{} `json:"values"`
}

// TODO: datasources

type TerraformState struct {
	Resources []TerraformResource
	// TODO: explicit provider handling
	// TODO: provider versions
	Providers []string
	// TODO: stack outputs?
}

func readTerraformState(filename string) (*TerraformState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// TODO: look into tf library to do this tfjson?

	var state struct {
		Values struct {
			RootModule struct {
				Resources    []TerraformResource `json:"resources,omitempty"`
				ChildModules []struct {
					Resources []TerraformResource `json:"resources,omitempty"`
				} `json:"child_modules,omitempty"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	resources := make([]TerraformResource, 0)
	resources = append(resources, state.Values.RootModule.Resources...)
	for _, childModule := range state.Values.RootModule.ChildModules {
		resources = append(resources, childModule.Resources...)
	}
	providerMap := make(map[string]struct{})
	for _, resource := range resources {
		providerMap[resource.ProviderName] = struct{}{}
	}
	providerList := make([]string, 0, len(providerMap))
	for provider := range providerMap {
		providerList = append(providerList, provider)
	}
	sort.Strings(providerList)
	return &TerraformState{
		Resources: resources,
		Providers: providerList,
	}, nil
}

type CtyResource struct {
	Type  string
	Value cty.Value
}

func resourceToCtyValue(resource *TerraformResource, resourceType valueshim.Type) (cty.Value, error) {
	data, err := json.Marshal(resource.Values)
	if err != nil {
		return cty.Value{}, err
	}
	ty, ok := valueshim.ToCtyType(resourceType)
	if !ok {
		return cty.Value{}, fmt.Errorf("expected cty-based Type implementation")
	}
	return ctyjson.Unmarshal(data, ty)
}
