package diagnostics

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
)

// ValidationError wraps diagnostics reported by shims (currently shim v2 and tf5).
type ValidationError struct {
	AttributePath cty.Path
	Summary       string
	Detail        string
}

// Last-resort error printout.
//
// To display nice error messages to Pulumi users the bridge should recognize ValidationError structs and reformat
// AttributePath in terms of the Pulumi provider as appropriate, so this method should not be called on paths that are
// expected to be user-visible.
//
// However this method is currently called in unrecoverable situations when underlying TF machinery fails. Opt to expose
// all the information here to facilitate debugging.
func (e ValidationError) Error() string {
	var buf bytes.Buffer
	if len(e.AttributePath) > 0 {
		fmt.Fprintf(&buf, "[")
		for i, p := range e.AttributePath {
			switch p := p.(type) {
			case cty.IndexStep:
				if p.Key.Type() == cty.String {
					fmt.Fprintf(&buf, "[%q]", p.Key.AsString())
				} else if p.Key.Type() == cty.Number {
					fmt.Fprintf(&buf, "[%v]", p.Key.AsBigFloat().String())
				}
			case cty.GetAttrStep:
				if i > 0 {
					fmt.Fprintf(&buf, ".")
				}
				fmt.Fprintf(&buf, p.Name)
			}
		}
		fmt.Fprintf(&buf, "] ")
	}
	if e.Detail != "" {
		fmt.Fprintf(&buf, "%s: %s", e.Summary, e.Detail)
	} else {
		fmt.Fprintf(&buf, e.Summary)
	}
	return buf.String()
}
