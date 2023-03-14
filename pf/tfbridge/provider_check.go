// Copyright 2016-2022, Pulumi Corporation.
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

package tfbridge

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Check validates the given resource inputs from the user program and computes checked inputs that fill out default
// values. The checked inputs are then passed to subsequent, Diff, Create, or Update.
func (p *provider) CheckWithContext(
	ctx context.Context,
	urn resource.URN,
	priorState resource.PropertyMap,
	inputs resource.PropertyMap,
	allowUnknowns bool,
	randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {

	ctx = initLogging(ctx)

	// TODO[pulumi/pulumi-terraform-bridge#822] ValidateResourceConfig
	checkedInputs := inputs.Copy()

	return checkedInputs, []plugin.CheckFailure{}, nil
}
