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
)

// Create allocates a new instance of the provided resource and returns its unique resource.ID.
func (p *Provider) Create(urn resource.URN, news resource.PropertyMap,
	timeout float64, preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return "", nil, 0, err
	}

	tfType := rh.schema.Type().TerraformType(ctx)

	if preview {
		// TODO clarify what to do here, how to handle preview
		// Create properly. For now match empirically observed
		// behavior.

		// Transcoding through DynamicValue achieves filtering
		// of properties to only retain what TF understands.
		plannedState, err := ConvertPropertyMapToDynamicValue(tfType.(tftypes.Object))(news)
		if err != nil {
			return "", nil, 0, err
		}
		recovered, err := ConvertDynamicValueToPropertyMap(tfType.(tftypes.Object))(plannedState)
		if err != nil {
			return "", nil, 0, err
		}

		return "", recovered, resource.StatusOK, nil
	}

	// priorState is nil since we are in Create
	priorState, err := tfprotov6.NewDynamicValue(tfType, tftypes.NewValue(tfType, nil))
	if err != nil {
		return "", nil, 0, err

	}

	// plannedState is simply news
	//
	// Note: that this conversion implicitly filters to only deal
	// with the fields specified in the tfType schema.
	plannedState, err := ConvertPropertyMapToDynamicValue(tfType.(tftypes.Object))(news)
	if err != nil {
		return "", nil, 0, err
	}

	req := tfprotov6.ApplyResourceChangeRequest{
		TypeName:     rh.terraformResourceName,
		PriorState:   &priorState,
		PlannedState: &plannedState,

		// TODO Config properly.
		//
		// See https://www.terraform.io/plugin/framework/accessing-values
		//
		// Provider may want to read resource configuration separately from the Plan. Need to clarify how these can be
		// different (perhaps .Config is as-written and excludes any computations performed by executing the program).
		// Currently it is not obvious where to find this data in Pulumi protocol.
		Config: &plannedState,

		// TODO PlannedPrivate
		// PlannedPrivate: []byte{},

		// TODO set ProviderMeta
		//
		// See https://www.terraform.io/internals/provider-meta
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &req)
	if err != nil {
		return "", nil, 0, err
	}

	// TODO handle resp.Diagnostics more than just detecting the
	// first error; handle warnings, process multiple errors.
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			prefix := ""
			if d.Attribute != nil {
				prefix = fmt.Sprintf("[%s] ", d.Attribute.String())
			}
			return "", nil, 0, fmt.Errorf("%s%s: %s", prefix, d.Summary, d.Detail)
		}
	}

	// TODO handle resp.Private field to save that state inside Pulumi state.

	createdState, err := ConvertDynamicValueToPropertyMap(tfType.(tftypes.Object))(*resp.NewState)
	if err != nil {
		return "", nil, 0, err
	}

	// TODO allocate ID properly
	createdID := resource.ID(createdState["id"].StringValue())

	return createdID, createdState, resource.StatusOK, nil
}
