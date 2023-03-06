package terraform

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf/tfdiags"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

// evaluateCountExpression is our standard mechanism for interpreting an
// expression given for a "count" argument on a resource or a module. This
// should be called during expansion in order to determine the final count
// value.
//
// evaluateCountExpression differs from evaluateCountExpressionValue by
// returning an error if the count value is not known, and converting the
// cty.Value to an integer.
func evaluateCountExpression(expr hcl.Expression, ctx EvalContext) (int, tfdiags.Diagnostics) {
	countVal, diags := evaluateCountExpressionValue(expr, ctx)
	if !countVal.IsKnown() {
		// Currently this is a rather bad outcome from a UX standpoint, since we have
		// no real mechanism to deal with this situation and all we can do is produce
		// an error message.
		// FIXME: In future, implement a built-in mechanism for deferring changes that
		// can't yet be predicted, and use it to guide the user through several
		// plan/apply steps until the desired configuration is eventually reached.
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid count argument",
			Detail:   `The "count" value depends on resource attributes that cannot be determined until apply, so Terraform cannot predict how many instances will be created. To work around this, use the -target argument to first apply only the resources that the count depends on.`,
			Subject:  expr.Range().Ptr(),

			// TODO: Also populate Expression and EvalContext in here, but
			// we can't easily do that right now because the hcl.EvalContext
			// (which is not the same as the ctx we have in scope here) is
			// hidden away inside evaluateCountExpressionValue.
			Extra: diagnosticCausedByUnknown(true),
		})
	}

	if countVal.IsNull() || !countVal.IsKnown() {
		return -1, diags
	}

	count, _ := countVal.AsBigFloat().Int64()
	return int(count), diags
}

// evaluateCountExpressionValue is like evaluateCountExpression
// except that it returns a cty.Value which must be a cty.Number and can be
// unknown.
func evaluateCountExpressionValue(expr hcl.Expression, ctx EvalContext) (cty.Value, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics
	nullCount := cty.NullVal(cty.Number)
	if expr == nil {
		return nullCount, nil
	}

	countVal, countDiags := ctx.EvaluateExpr(expr, cty.Number, nil)
	diags = diags.Append(countDiags)
	if diags.HasErrors() {
		return nullCount, diags
	}

	// Unmark the count value, sensitive values are allowed in count but not for_each,
	// as using it here will not disclose the sensitive value
	countVal, _ = countVal.Unmark()

	switch {
	case countVal.IsNull():
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid count argument",
			Detail:   `The given "count" argument value is null. An integer is required.`,
			Subject:  expr.Range().Ptr(),
		})
		return nullCount, diags

	case !countVal.IsKnown():
		return cty.UnknownVal(cty.Number), diags
	}

	var count int
	err := gocty.FromCtyValue(countVal, &count)
	if err != nil {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid count argument",
			Detail:   fmt.Sprintf(`The given "count" argument value is unsuitable: %s.`, err),
			Subject:  expr.Range().Ptr(),
		})
		return nullCount, diags
	}
	if count < 0 {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid count argument",
			Detail:   `The given "count" argument value is unsuitable: must be greater than or equal to zero.`,
			Subject:  expr.Range().Ptr(),
		})
		return nullCount, diags
	}

	return countVal, diags
}
