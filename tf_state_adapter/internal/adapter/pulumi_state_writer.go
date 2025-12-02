package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"gopkg.in/yaml.v3"
)

func makeUrn(stackName, projectName, typeName, resourceName string) resource.URN {
	return resource.URN(fmt.Sprintf("urn:pulumi:%s::%s::%s::%s", stackName, projectName, typeName, resourceName))
}

func MakeDeployment(state *PulumiState, outputFolder string) (apitype.DeploymentV3, error) {
	if outputFolder == "" {
		var err error
		outputFolder, err = os.MkdirTemp("", "pulumi-state-adapter-")
		if err != nil {
			return apitype.DeploymentV3{}, fmt.Errorf("failed to create temporary output folder: %w", err)
		}

		// random name
		projectName := "pulumi-state-adapter-" + strconv.Itoa(rand.Intn(1000000))

		// write a Pulumi.yaml file
		pulumiYaml := map[string]any{
			"name":    projectName,
			"runtime": "nodejs",
		}
		bytes, err := yaml.Marshal(pulumiYaml)
		if err != nil {
			return apitype.DeploymentV3{}, fmt.Errorf("failed to marshal Pulumi.yaml: %w", err)
		}

		err = os.WriteFile(filepath.Join(outputFolder, "Pulumi.yaml"), bytes, 0o600)
		if err != nil {
			return apitype.DeploymentV3{}, fmt.Errorf("failed to write Pulumi.yaml: %w", err)
		}
	}

	stackName := "dev"

	workspace, err := auto.NewLocalWorkspace(context.Background(), auto.WorkDir(outputFolder))
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to create workspace: %w", err)
	}

	projectSettings, err := workspace.ProjectSettings(context.Background())
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to get project settings: %w", err)
	}

	projectName := string(projectSettings.Name)

	// err = workspace.CreateStack(context.Background(), stackName)
	// if err != nil {
	// 	return apitype.DeploymentV3{}, fmt.Errorf("failed to create stack: %w", err)
	// }

	stack, err := workspace.ExportStack(context.Background(), stackName)
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to export stack: %w", err)
	}

	deployment := apitype.DeploymentV3{}
	err = json.Unmarshal(stack.Deployment, &deployment)
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to unmarshal stack deployment: %w", err)
	}

	contract.Assertf(len(deployment.Resources) == 1, "expected stack resource in state, got %d", len(deployment.Resources))

	// stackResource := apitype.ResourceV3{
	// 	URN:    makeUrn(stackName, projectName, "pulumi:pulumi:Stack", projectName),
	// 	Custom: false,
	// 	Type:   tokens.Type("pulumi:pulumi:Stack"),
	// }
	// deployment.Resources = append(deployment.Resources, stackResource)
	stackResource := deployment.Resources[0]

	now := time.Now()

	providerState := state.Providers[0]
	provider := apitype.ResourceV3{
		URN:      makeUrn(stackName, projectName, providerState.Type, providerState.Name),
		Custom:   providerState.Custom,
		ID:       resource.ID(providerState.ID),
		Type:     tokens.Type(providerState.Type),
		Inputs:   providerState.Inputs.Mappable(),
		Outputs:  providerState.Outputs.Mappable(),
		Created:  &now,
		Modified: &now,
	}
	deployment.Resources = append(deployment.Resources, provider)

	for _, res := range state.Resources {
		deployment.Resources = append(deployment.Resources, apitype.ResourceV3{
			URN:      makeUrn(stackName, projectName, res.Type, res.Name),
			Custom:   res.Custom,
			ID:       resource.ID(res.ID),
			Type:     tokens.Type(res.Type),
			Inputs:   res.Inputs.Mappable(),
			Outputs:  res.Outputs.Mappable(),
			Parent:   resource.URN(stackResource.URN),
			Provider: string(provider.URN) + "::" + string(provider.ID),
			Created:  &now,
			Modified: &now,
		})
	}

	err = workspace.ImportStack(context.Background(), stackName, stack)
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to import stack: %w", err)
	}

	return deployment, nil
}
