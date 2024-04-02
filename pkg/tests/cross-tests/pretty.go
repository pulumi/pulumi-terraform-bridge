package crosstests

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Large printouts of tftypes.Value are very difficult to read when debugging the tests, especially because of all the
// extraneous type information printed. This wrapper is a work in progress to implement better pretty-printing.
type prettyValueWrapper struct {
	inner tftypes.Value
}

func newPrettyValueWrapper(v tftypes.Value) prettyValueWrapper {
	return prettyValueWrapper{v}
}

func (s prettyValueWrapper) Value() tftypes.Value {
	return s.inner
}

// This is not yet valid Go syntax, but when rapid.Draw is used to pull a value it calls GoString and logs the result,
// which is the primary way to interact with the printout, so the code opts to implement this.
func (s prettyValueWrapper) GoString() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<<<\n")
	tftypes.Walk(s.inner, func(ap *tftypes.AttributePath, v tftypes.Value) (bool, error) {
		switch {
		case v.Type().Is(tftypes.Object{}) || v.Type().Is(tftypes.Set{}) ||
			v.Type().Is(tftypes.Map{}) || v.Type().Is(tftypes.List{}):
			return true, nil
		default:
			fmt.Fprintf(&buf, "%s: %s\n", ap.String(), v.String())
			return true, nil
		}
	})
	fmt.Fprintf(&buf, ">>>\n")
	return buf.String() + ":" + fmt.Sprintf("%#v", s.inner)
}
