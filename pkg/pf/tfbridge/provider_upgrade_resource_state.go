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
	"errors"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/pfutils"
)

// Wraps running state migration via the underlying TF UpgradeResourceState method.
func (p *provider) UpgradeResourceState(
	ctx context.Context,
	rh *resourceHandle,
	st *rawResourceState,
) (*upgradedResourceState, error) {
	tfType := rh.schema.Type(ctx).(tftypes.Object)
	req := &tfprotov6.UpgradeResourceStateRequest{
		TypeName: rh.terraformResourceName,
		Version:  st.TFSchemaVersion,
	}

	if st.RawState != nil {
		req.RawState = st.RawState
	} else if st.Value != nil {
		rawState, err := pfutils.NewRawState(tfType, *st.Value)
		if err != nil {
			return nil, fmt.Errorf("error calling NewRawState: %w", err)
		}
		req.RawState = rawState
	} else {
		contract.Failf("rawResourceState should have either RawState or Value set")
		return nil, errors.New("Contract failure")
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
		return nil, fmt.Errorf("error unmarshalling the response from UpgradeResourceState: %w", err)
	}

	// Downgrade float precision to 53. This is important because pulumi.PropertyValue stores float64, and
	// tftypes.Value originating from Pulumi would have this precision, but the native precision coming from
	// UpgradeResourceState is 512. Mismatches in precision may lead to spurious diffs.
	v, err = tftypes.Transform(v, func(ap *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if v.IsKnown() && !v.IsNull() && v.Type().Is(tftypes.Number) {
			var n big.Float
			err := v.As(&n)
			contract.AssertNoErrorf(err, "Values of tftypes.Number type should unpack to big.Float")
			if n.Prec() != 53 {
				v2 := tftypes.NewValue(tftypes.Number, new(big.Float).Copy(&n).SetPrec(53))
				return v2, nil
			}
		}
		return v, nil
	})
	contract.AssertNoErrorf(err, "float precision downgrade transform should not fail")

	return &upgradedResourceState{&resourceState{
		TFSchemaVersion: rh.schema.ResourceSchemaVersion(),
		Value:           v,
		Private:         st.Private,
	}}, nil
}
