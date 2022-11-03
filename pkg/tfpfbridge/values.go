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
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func applyChanges(old, changes tftypes.Value) (r tftypes.Value, reterr error) {
	if !old.Type().Equal(changes.Type()) {
		return r, fmt.Errorf("applyChanges expecting old.Type(%s) and changes.Type(%s) to be equal",
			old.Type().String(), changes.Type().String())
	}
	diffs, err := old.Diff(changes)
	if err != nil {
		return r, fmt.Errorf("applyChanges failed to Diff: %w", err)
	}
	res, err := applyDiffs(diffs, old)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("applyChanges failed: %w", err)
	}
	return res, nil
}

func applyDiffs(diffs []tftypes.ValueDiff, v tftypes.Value) (tftypes.Value, error) {
	res, err := tftypes.Transform(v, func(p *tftypes.AttributePath, old tftypes.Value) (tftypes.Value, error) {
		for _, d := range diffs {
			if d.Path.Equal(p) {
				return d.Value2.Copy(), nil
			}
		}
		return old, nil
	})
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("applyDiffs failed to Transform: %w", err)
	}
	return res, nil
}
