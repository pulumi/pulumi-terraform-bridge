// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stringplanmodifiers

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// RequiresReplaceUnlessEmptyStringToNull returns a
// resource.RequiresReplaceIfFunc that returns true unless the change is from
// an empty string to null. This plan modifier allows practitioners to fix
// their configurations in this situation without replacing the resource.
//
// For example, version 3.4.2 errantly setup the random_password and
// random_string resource override_special attributes with an empty string
// default value plan modifier.
func RequiresReplaceUnlessEmptyStringToNull() stringplanmodifier.RequiresReplaceIfFunc {
	return func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		// If the configuration is unknown, this cannot be sure what to do yet.
		if req.ConfigValue.IsUnknown() {
			resp.RequiresReplace = false
			return
		}

		// If the configuration is not null or the state is already null,
		// replace the resource.
		if !req.ConfigValue.IsNull() || req.StateValue.IsNull() {
			resp.RequiresReplace = true
			return
		}

		// If the state is not an empty string, replace the resource.
		if req.StateValue.ValueString() != "" {
			resp.RequiresReplace = true
			return
		}

		resp.RequiresReplace = false
	}
}
