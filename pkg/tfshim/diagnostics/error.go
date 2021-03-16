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
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Summary, e.Detail)
	}
	return e.Summary
}
