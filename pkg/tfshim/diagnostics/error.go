package diagnostics

import (
	"fmt"
	"github.com/hashicorp/go-cty/cty"
)

type Error struct {
	AttributePath cty.Path
	Summary       string
	Detail        string
}

func (e Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Summary, e.Detail)
	}
	return e.Summary
}
