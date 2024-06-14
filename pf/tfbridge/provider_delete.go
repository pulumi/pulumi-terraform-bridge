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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
)

func (p *provider) Delete(
	ctx context.Context,
	req plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	ctx = p.initLogging(ctx, p.logSink, req.URN)

	rh, err := p.resourceHandle(ctx, req.URN)
	if err != nil {
		return plugin.DeleteResponse{Status: resource.StatusOK}, err
	}

	props, err := transformFromState(ctx, rh, req.Outputs)
	if err != nil {
		return plugin.DeleteResponse{Status: resource.StatusOK}, err
	}

	tfType := rh.schema.Type(ctx).(tftypes.Object)

	priorState, err := convert.EncodePropertyMapToDynamic(rh.encoder, tfType, props)
	if err != nil {
		return plugin.DeleteResponse{Status: resource.StatusOK}, err
	}

	// terraform-plugin-framework recognizes PlannedState=nil ApplyResourceChangeRequest request as DELETE.
	//
	//nolint:lll // See
	// https://github.com/hashicorp/terraform-plugin-framework/blob/ce2519cf40d45d28eebd81776019e68d1bddca6f/internal/fwserver/server_applyresourcechange.go#L63
	tfReq := tfprotov6.ApplyResourceChangeRequest{
		TypeName:   rh.terraformResourceName,
		PriorState: priorState,
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &tfReq)
	if err != nil {
		return plugin.DeleteResponse{Status: resource.StatusOK}, err
	}

	// NOTE: no need to handle resp.Private in Delete.

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, err
	}

	// In one example that was tested, resp.NewState after a
	// successful delete seem to have a record with all null
	// values. Seems safe to simply ignore it.

	return plugin.DeleteResponse{Status: resource.StatusOK}, nil
}
