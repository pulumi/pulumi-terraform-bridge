package diagnostics

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
)

// ValidationWarning wraps diagnostic warnings reported by shims, preserving structured
// attribute path information so that consumers can translate property names to Pulumi conventions.
type ValidationWarning struct {
	AttributePath cty.Path
	Summary       string
	Detail        string
}

// String renders the warning for display. Consumers should prefer to translate the AttributePath
// to Pulumi property names using formatValidationWarning before falling back to this method.
func (w ValidationWarning) String() string {
	var buf bytes.Buffer
	if len(w.AttributePath) > 0 {
		fmt.Fprintf(&buf, "[")
		for i, p := range w.AttributePath {
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
				fmt.Fprintf(&buf, "%s", p.Name)
			}
		}
		fmt.Fprintf(&buf, "] ")
	}
	if w.Detail != "" {
		fmt.Fprintf(&buf, "%s: %s", w.Summary, w.Detail)
	} else {
		fmt.Fprintf(&buf, "%s", w.Summary)
	}
	return buf.String()
}
