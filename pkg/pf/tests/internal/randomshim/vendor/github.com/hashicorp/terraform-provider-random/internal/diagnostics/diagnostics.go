// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package diagnostics

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

const RetryMsg = "Retry the Terraform operation. If the error still occurs or happens regularly, please contact the provider developer with hardware and operating system information.\n\n"

func RandomReadError(errMsg string) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddError(
		"Random Read Error",
		"While attempting to generate a random value for this resource, a read error was generated.\n\n"+
			RetryMsg+
			fmt.Sprintf("Original Error: %s", errMsg),
	)

	return diags
}

func HashGenerationError(errMsg string) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddError(
		"Hash Generation Error",
		"While attempting to generate a hash from the password an error occurred.\n\n"+
			"Verify that the state contains a populated 'result' field, using 'terraform state show', and retry the operation\n\n"+
			fmt.Sprintf("Original Error: %s", errMsg),
	)

	return diags
}

func RandomnessGenerationError(errMsg string) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddError(
		"Randomness Generation Error",
		"While attempting to generate a random value for this resource, an insufficient number of random bytes were generated.\n\n"+
			RetryMsg+
			fmt.Sprintf("Original Error: %s", errMsg),
	)

	return diags
}
