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
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *Provider) Diff(urn resource.URN, id resource.ID, olds resource.PropertyMap,
	news resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {

	ctx := context.TODO()

	resources, err := p.resources(ctx)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	typeName, err := p.terraformResourceName(urn.Type())
	if err != nil {
		return plugin.DiffResult{}, err
	}

	var schema tfsdk.Schema = resources.schemaByTypeName[typeName]

	tfType := schema.Type().TerraformType(ctx)

	priorState, err := ConvertPropertyMapToDynamicValue(tfType.(tftypes.Object))(olds)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	proposedNewState, err := ConvertPropertyMapToDynamicValue(tfType.(tftypes.Object))(news)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	planReq := tfprotov6.PlanResourceChangeRequest{
		TypeName:         typeName,
		PriorState:       &priorState,
		ProposedNewState: &proposedNewState,

		// TODO this does not seem right. In what
		// circumstances Config differes from ProposedNewState
		// in Terraform and how should this work in Pulumi?
		Config: &proposedNewState,

		// TODO PriorPrivate
		// TODO ProviderMeta
	}

	planResp, err := p.tfServer.PlanResourceChange(ctx, &planReq)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	// here's what bridge code did before:

	// state := MakeTerraformState(olds)
	// config  := MakeTerraformConfig(news)
	// diff := p.tf.Diff(.., state, config)

	// doIgnoreChanges(ignoreChanges)

	// compute stables including treating specific properties as stable
	// figure out deleteBeforeReplace

	// makeDetailedDiff()

	// p.tf.Diff bottoms out at r.SimpleDiff() from
	// terraform-plugin-sdk/v2/helper/schema

	diags := []string{}
	for _, diag := range planResp.Diagnostics {
		diags = append(diags, diag.Summary, diag.Detail)
	}

	plannedState, err := planResp.PlannedState.Unmarshal(tfType)
	if err != nil {
		return plugin.DiffResult{}, err
	}

	panic(fmt.Sprintf("TODO DIFF: diags=%s urn=%s plannedState=%v REQ=%v olds=%v schema=%v",
		strings.Join(diags, ","),
		urn,
		plannedState.String(), planReq, olds, schema))
}
