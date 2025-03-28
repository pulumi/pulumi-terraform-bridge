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

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
)

func (p *provider) DeleteWithContext(
	ctx context.Context,
	urn resource.URN,
	id resource.ID,
	_ resource.PropertyMap, /* inputs */
	outputs resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return resource.StatusOK, err
	}

	props, err := transformFromState(ctx, rh, outputs)
	if err != nil {
		return resource.StatusOK, err
	}

	tfType := rh.schema.Type(ctx).(tftypes.Object)

	priorState, err := convert.EncodePropertyMapToDynamic(rh.encoder, tfType, props)
	if err != nil {
		return resource.StatusOK, err
	}

	nilState, err := tfprotov6.NewDynamicValue(tfType, tftypes.NewValue(tfType, nil))
	contract.AssertNoErrorf(err, "nil is always a valid value to marshal to a dynamic state")

	// terraform-plugin-framework recognizes PlannedState=nil ApplyResourceChangeRequest request as DELETE.
	//
	//nolint:lll // See
	// https://github.com/hashicorp/terraform-plugin-framework/blob/ce2519cf40d45d28eebd81776019e68d1bddca6f/internal/fwserver/server_applyresourcechange.go#L63
	req := tfprotov6.ApplyResourceChangeRequest{
		TypeName:     rh.terraformResourceName,
		PriorState:   priorState,
		PlannedState: &nilState,
		Config:       &nilState,
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &req)
	if err != nil {
		return resource.StatusOK, err
	}

	// NOTE: no need to handle resp.Private in Delete.

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return resource.StatusPartialFailure, err
	}

	// In one example that was tested, resp.NewState after a
	// successful delete seem to have a record with all null
	// values. Seems safe to simply ignore it.

	return resource.StatusOK, nil
}
