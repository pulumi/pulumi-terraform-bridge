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

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
)

func (p *Provider) Delete(urn resource.URN, id resource.ID,
	props resource.PropertyMap, timeout float64) (resource.Status, error) {

	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return resource.StatusOK, err
	}

	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)

	priorState, err := convert.EncodePropertyMapToDynamic(rh.encoder, tfType, props)
	if err != nil {
		return resource.StatusOK, err
	}

	// terraform-plugin-framework recognizes PlannedState=nil ApplyResourceChangeRequest request DELETE.
	//
	// See https://github.com/hashicorp/terraform-plugin-framework/blob/ce2519cf40d45d28eebd81776019e68d1bddca6f/internal/fwserver/server_applyresourcechange.go#L63
	req := tfprotov6.ApplyResourceChangeRequest{
		TypeName:   rh.terraformResourceName,
		PriorState: priorState,
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &req)
	if err != nil {
		return resource.StatusOK, err
	}
	// TODO handle resp.Private

	// TODO handle resp.Diagnostics more than just detecting the
	// first error; handle warnings, process multiple errors.
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			prefix := ""
			if d.Attribute != nil {
				prefix = fmt.Sprintf("[%s] ", d.Attribute.String())
			}
			return resource.StatusOK, fmt.Errorf("%s%s: %s", prefix, d.Summary, d.Detail)
		}
	}

	// In one example that was tested, resp.NewState after a
	// successful delete seem to have a record with all null
	// values. Seems safe to simply ignore it.

	return resource.StatusOK, nil
}
