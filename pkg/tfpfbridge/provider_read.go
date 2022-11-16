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

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Read the current live state associated with a resource. Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties. If the resource
// is missing (for instance, because it has been deleted), the resulting property map will be nil.
func (p *Provider) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

	// TODO test for a resource that is not found

	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)

	// Note: that this conversion implicitly filters to only deal
	// with the fields specified in the tfType schema.
	currentState, err := ConvertPropertyMapToDynamicValue(tfType)(state)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	req := tfprotov6.ReadResourceRequest{
		TypeName:     rh.terraformResourceName,
		CurrentState: &currentState,
	}

	// TODO Set ProviderMeta
	// TODO Set Private

	resp, err := p.tfServer.ReadResource(ctx, &req)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	// TODO handle resp.Diagnostics more than just detecting the
	// first error; handle warnings, process multiple errors.
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			prefix := ""
			if d.Attribute != nil {
				prefix = fmt.Sprintf("[%s] ", d.Attribute.String())
			}
			return plugin.ReadResult{}, 0,
				fmt.Errorf("%s%s: %s", prefix, d.Summary, d.Detail)
		}
	}

	// TODO handle resp.Private

	if resp.NewState == nil {
		return plugin.ReadResult{}, resource.StatusUnknown, nil
	}

	readResourceStateValue, err := resp.NewState.Unmarshal(tfType)
	if err != nil {
		return plugin.ReadResult{}, resource.StatusUnknown, nil
	}

	readState, err := ConvertTFValueToPropertyMap(tfType)(readResourceStateValue)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	readID, err := rh.idExtractor(readResourceStateValue)
	if err != nil {
		return plugin.ReadResult{}, 0, err
	}

	return plugin.ReadResult{
		ID: resource.ID(readID),
		// TODO support populating inputs, see extractInputsFromOutputs in the prod bridge.
		Inputs:  nil,
		Outputs: readState,
	}, resource.StatusOK, nil
}
