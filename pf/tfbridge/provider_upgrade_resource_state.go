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

package tfbridge

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
)

// Wraps running state migration via the underlying TF UpgradeResourceState method.
func (p *provider) UpgradeResourceState(
	ctx context.Context,
	rh *resourceHandle,
	st *resourceState,
) (*upgradedResourceState, error) {
	if st.TFSchemaVersion == 0 || st.TFSchemaVersion >= rh.schema.ResourceSchemaVersion() {
		return &upgradedResourceState{st}, nil
	}
	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)
	rawState, err := pfutils.NewRawState(tfType, st.Value)
	req := &tfprotov6.UpgradeResourceStateRequest{
		TypeName: rh.terraformResourceName,
		Version:  st.TFSchemaVersion,
		RawState: rawState,
	}
	resp, err := p.tfServer.UpgradeResourceState(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error calling UpgradeResourceState: %w", err)
	}
	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return nil, err
	}
	v, err := resp.UpgradedState.Unmarshal(tfType)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling the repsonse from UpgradeResourceState: %w", err)
	}
	return &upgradedResourceState{&resourceState{
		TFSchemaVersion: rh.schema.ResourceSchemaVersion(),
		Value:           v,
	}}, nil
}
