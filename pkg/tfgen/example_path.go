// Copyright 2016-2023, Pulumi Corporation.
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

package tfgen

import (
	"fmt"
)

// examplePath explains where in Pulumi Schema the example is found. For instance,
// "#/resources/aws:acm/certificate:Certificate/arn" would encode that the example pertains to the arn property of the
// Certificate resource.
type examplePath struct {
	token    string
	fullPath string
}

func (path examplePath) Token() string {
	return path.token
}

func (path examplePath) String() string {
	return path.fullPath
}

func (path examplePath) Property(pulumiName string) examplePath {
	return examplePath{
		fullPath: fmt.Sprintf("%s/%s", path.String(), pulumiName),
		token:    path.token,
	}
}

func (path examplePath) Inputs() examplePath {
	return examplePath{
		token:    path.token,
		fullPath: fmt.Sprintf("%s/inputs", path.String()),
	}
}

func (path examplePath) StateInputs() examplePath {
	return examplePath{
		token:    path.token,
		fullPath: fmt.Sprintf("%s/stateInputs", path.String()),
	}
}

func (path examplePath) Outputs() examplePath {
	return examplePath{
		token:    path.token,
		fullPath: fmt.Sprintf("%s/outputs", path.String()),
	}
}

func newExamplePathForResource(pulumiResourceToken string) examplePath {
	return examplePath{
		token:    pulumiResourceToken,
		fullPath: fmt.Sprintf("#/resources/" + pulumiResourceToken),
	}
}

func newExamplePathForFunction(pulumiFuncToken string) examplePath {
	return examplePath{
		token:    pulumiFuncToken,
		fullPath: fmt.Sprintf("#/functions/" + pulumiFuncToken),
	}
}

func newExamplePathForProvider() examplePath {
	return examplePath{fullPath: "#/provider"}
}

func newExamplePathForNamedType(pulumiTypeToken string) examplePath {
	return examplePath{
		token:    pulumiTypeToken,
		fullPath: "#/types/" + pulumiTypeToken,
	}
}

func newExamplePathForProviderConfigVariable(pulumiConfigVariableName string) examplePath {
	// This is a bit odd that it is unprefixed; preserving for backwards compatibility.
	return examplePath{fullPath: pulumiConfigVariableName}
}
