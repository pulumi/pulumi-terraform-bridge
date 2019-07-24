// Copyright 2016-2019, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"io/ioutil"
	"log"

	structpb "github.com/golang/protobuf/ptypes/struct"
	be "github.com/hashicorp/terraform/backend"
	backendInit "github.com/hashicorp/terraform/backend/init"
	"github.com/pulumi/pulumi/sdk/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func remoteStateReferenceRead(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	// Prevent Terraform from logging minutia
	log.SetOutput(ioutil.Discard)

	// Pull the backendType out of the backendConfig, ensure it's a string
	backendTypePB, hasBackendType := req.GetProperties().GetFields()["backendType"]
	if !hasBackendType {
		return nil, status.Errorf(codes.InvalidArgument,
			"missing required property %q", "backendType")
	}
	if _, isString := backendTypePB.Kind.(*structpb.Value_StringValue); !isString {
		return nil, status.Errorf(codes.InvalidArgument,
			"expected a string value for property %q", "backendType")
	}
	backendType := backendTypePB.GetStringValue()

	// Ensure the backendType is known about by Terraform
	backendInitFn := backendInit.Backend(backendType)
	if backendInitFn == nil {
		return nil, status.Errorf(codes.InvalidArgument,
			"unsupported backend type %q", backendType)
	}

	// If we have a workspace specified, get the value for that. Use the default otherwise
	workspaceName := be.DefaultStateName
	if workspacePB, hasWorkspaceTypePB := req.GetProperties().GetFields()["workspace"]; hasWorkspaceTypePB {
		if _, isString := workspacePB.Kind.(*structpb.Value_StringValue); !isString {
			return nil, status.Errorf(codes.InvalidArgument,
				"expected a string value for property %q", "workspace")
		}

		workspaceName = workspacePB.GetStringValue()
	}

	// Convert our backendConfig struct to something usable with the backend configuration schema
	terraformNamedNews := structpbNamesPulumiToTerraform(req.GetProperties())

	// Delete fields which are known not to be part of the Terraform remote state config
	// which will cause unknown field failures if passed to a go-cty CoerceValue function
	delete(terraformNamedNews.GetFields(), "backend_type")
	delete(terraformNamedNews.GetFields(), "workspace")
	delete(terraformNamedNews.GetFields(), "outputs")

	backendConfigCty, err := structpbToCtyObject(terraformNamedNews)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error mapping config from Pulumi format to cty: %s", err)
	}

	// Get the configuration schema from the backend
	backend := backendInitFn()

	// Attempt to coerce our config object into the config schema types - note errors
	backendConfigCoerced, err := backend.ConfigSchema().CoerceValue(backendConfigCty)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error coercing config from Pulumi format to cty: %s", err)
	}

	// Attempt to prepare the backend with configuration, returning any diagnostics to the engine
	preparedBackendConfig, diagnostics := backend.PrepareConfig(backendConfigCoerced)
	if diagnostics.HasErrors() {
		return nil, status.Errorf(codes.Internal, "error preparing config: %s", diagnostics.Err())
	}

	// Actually prepare the backend with the valid configuration
	diagnostics = backend.Configure(preparedBackendConfig)
	if diagnostics.HasErrors() {
		return nil, status.Errorf(codes.InvalidArgument, "error in backend configuration: %s",
			diagnostics.ErrWithWarnings())
	}

	// Get the state manager from the backend for the appropriate workspace
	stateManager, err := backend.StateMgr(workspaceName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error constructing backend state manager: %s", err)
	}

	// Refresh the state
	if err := stateManager.RefreshState(); err != nil {
		return nil, status.Errorf(codes.NotFound, "error refreshing Terraform state: %s", err)
	}

	// Check the state isn't empty
	state := stateManager.State()
	if state == nil {
		return nil, status.Error(codes.NotFound, "remote state not found")
	}

	// Get the root module outputs and process them from a map of string to cty.Value into a structpb
	outputsCty := state.RootModule().OutputValues
	outputsStructpb, err := outputsToStructpb(outputsCty)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error converting Terraform outputs: %s", err)
	}

	// Construct our properties based on the outputs
	req.GetProperties().Fields["outputs"] = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: outputsStructpb,
		},
	}

	// Return a successful response to the engine
	return &pulumirpc.ReadResponse{
		Id:         req.Id,
		Inputs:     req.Inputs,
		Properties: req.GetProperties(),
	}, nil
}
