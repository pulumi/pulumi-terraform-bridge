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
type examplePath string

func (path examplePath) String() string {
	return string(path)
}

func (path examplePath) Property(pulumiName string) examplePath {
	return examplePath(fmt.Sprintf("%s/%s", string(path), pulumiName))
}

func (path examplePath) Inputs() examplePath {
	return examplePath(fmt.Sprintf("%s/inputs", string(path)))
}

func (path examplePath) Outputs() examplePath {
	return examplePath(fmt.Sprintf("%s/outputs", string(path)))
}

func newExamplePathForResource(pulumiResourceToken string) examplePath {
	return examplePath(fmt.Sprintf("#/resources/" + pulumiResourceToken))
}

func newExamplePathForFunction(pulumiFuncToken string) examplePath {
	return examplePath(fmt.Sprintf("#/functions/" + pulumiFuncToken))
}

func newExamplePathForProvider() examplePath {
	return examplePath("#/provider")
}

func newExamplePathForNamedType(pulumiTypeToken string) examplePath {
	return examplePath("#/types/" + pulumiTypeToken)
}

func newExamplePathForProviderConfigVariable(pulumiConfigVariableName string) examplePath {
	// This is a bit odd that it is unprefixed; preserving for backwards compatibility.
	return examplePath(pulumiConfigVariableName)
}
