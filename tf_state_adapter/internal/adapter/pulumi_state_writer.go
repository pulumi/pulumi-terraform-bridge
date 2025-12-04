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

func createProgram(outputFolder string) error {
	// random name
	projectName := "pulumi-state-adapter-" + strconv.Itoa(rand.Intn(1000000))

	// write a Pulumi.yaml file
	pulumiYaml := map[string]any{
		"name":    projectName,
		"runtime": "nodejs",
	}
	bytes, err := yaml.Marshal(pulumiYaml)
	if err != nil {
		return fmt.Errorf("failed to marshal Pulumi.yaml: %w", err)
	}

	err = os.WriteFile(filepath.Join(outputFolder, "Pulumi.yaml"), bytes, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write Pulumi.yaml: %w", err)
	}

	packageJson := map[string]any{
		"name": "pulumi_aws_tf_conv2",
		"main": "index.ts",
		"devDependencies": map[string]any{
			"@types/node": "^18",
			"typescript":  "^5.0.0",
		},
		"dependencies": map[string]any{
			"@pulumi/aws":    "7.11.1",
			"@pulumi/pulumi": "^3.113.0",
		},
	}

	bytes, err = json.Marshal(packageJson)
	if err != nil {
		return fmt.Errorf("failed to marshal package.json: %w", err)
	}

	err = os.WriteFile(filepath.Join(outputFolder, "package.json"), bytes, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write package.json: %w", err)
	}

	// create an index.ts file
	indexTs := `
			import * as pulumi from "@pulumi/pulumi";
			import * as aws from "@pulumi/aws";
			`
	err = os.WriteFile(filepath.Join(outputFolder, "index.ts"), []byte(indexTs), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write index.ts: %w", err)
	}

	return nil
}

func MakeDeployment(state *PulumiState, outputFolder string) (apitype.DeploymentV3, error) {
	ctx := context.Background()
	if outputFolder == "" {
		var err error
		outputFolder, err = os.MkdirTemp("", "pulumi-state-adapter-")
		if err != nil {
			return apitype.DeploymentV3{}, fmt.Errorf("failed to create temporary output folder: %w", err)
		}

		// TODO: Figure out how to run pulumi new with automation API.
		err = createProgram(outputFolder)
		if err != nil {
			return apitype.DeploymentV3{}, fmt.Errorf("failed to create program: %w", err)
		}
	}

	stackName := "dev"

	workspace, err := auto.NewLocalWorkspace(ctx, auto.WorkDir(outputFolder))
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to create workspace: %w", err)
	}

	projectSettings, err := workspace.ProjectSettings(ctx)
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to get project settings: %w", err)
	}

	projectName := string(projectSettings.Name)

	// check if the stack already exists
	err = workspace.SelectStack(ctx, stackName)
	if err != nil {
		workspace.Install(ctx, &auto.InstallOptions{})
		s, err := auto.UpsertStackLocalSource(ctx, stackName, outputFolder)
		if err != nil {
			return apitype.DeploymentV3{}, fmt.Errorf("failed to create stack: %w", err)
		}
		_, err = s.Up(ctx)
		if err != nil {
			return apitype.DeploymentV3{}, fmt.Errorf("failed to run up: %w", err)
		}
	}

	stack, err := workspace.ExportStack(ctx, stackName)
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
		Custom:   true,
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
			Custom:   true,
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

	err = workspace.ImportStack(ctx, stackName, stack)
	if err != nil {
		return apitype.DeploymentV3{}, fmt.Errorf("failed to import stack: %w", err)
	}

	return deployment, nil
}
