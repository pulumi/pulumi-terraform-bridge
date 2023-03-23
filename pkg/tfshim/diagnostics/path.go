package diagnostics

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
)

// FormatCtyPath is a helper function to produce a user-friendly string
// representation of a cty.Path. The result uses a syntax similar to the
// HCL expression language in the hope of it being familiar to users.
//
// This code has been cribbed from [1].
// [1]: https://github.com/terraform-linters/tflint/blob/ec43cd65cd943d71a48419a72bb8e20e6559dfd2/terraform/tfdiags/config_traversals.go#L14
func formatCtyPath(path cty.Path) string {
	var buf bytes.Buffer
	for _, step := range path {
		switch ts := step.(type) {
		case cty.GetAttrStep:
			fmt.Fprintf(&buf, ".%s", ts.Name)
		case cty.IndexStep:
			buf.WriteByte('[')
			key := ts.Key
			keyTy := key.Type()
			switch {
			case key.IsNull():
				buf.WriteString("null")
			case !key.IsKnown():
				buf.WriteString("(not yet known)")
			case keyTy == cty.Number:
				bf := key.AsBigFloat()
				buf.WriteString(bf.Text('g', -1))
			case keyTy == cty.String:
				buf.WriteString(strconv.Quote(key.AsString()))
			default:
				buf.WriteString("...")
			}
			buf.WriteByte(']')
		}
	}
	return buf.String()
}
