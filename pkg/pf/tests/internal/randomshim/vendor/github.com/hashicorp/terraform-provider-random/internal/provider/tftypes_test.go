// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func testTftypesValueAtPath(value tftypes.Value, path *tftypes.AttributePath) (tftypes.Value, error) {
	valueAtPathRaw, remaining, err := tftypes.WalkAttributePath(value, path)

	if err != nil {
		return tftypes.Value{}, fmt.Errorf("unexpected error getting %s: %s (%s remaining)", path, err, remaining)
	}

	valueAtPath, ok := valueAtPathRaw.(tftypes.Value)

	if !ok {
		return tftypes.Value{}, fmt.Errorf("unexpected type converting %s to tftypes.Value, got: %T", path, valueAtPathRaw)
	}

	return valueAtPath, nil
}
