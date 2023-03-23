package diagnostics

import (
	"fmt"
	"github.com/hashicorp/go-cty/cty"
)

// ValidationError wraps validation errors reported by shims (currently shim v2 and tf5)
type ValidationError struct {
	AttributePath cty.Path
	Summary       string
	Detail        string
}

func (e ValidationError) Error() string {
	msg := e.Summary
	if len(e.AttributePath) > 0 {
		msg = fmt.Sprintf("%s %q", msg, formatCtyPath(e.AttributePath))
	}

	if e.Detail != "" {
		msg = fmt.Sprintf("%s: %s", msg, e.Detail)
	}

	return msg
}
