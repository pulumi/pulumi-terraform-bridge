package rawstate

import (
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Builder struct {
	v any
}

func (x Builder) Build() json.RawMessage {
	j, err := json.Marshal(x.v)
	contract.AssertNoErrorf(err, "Build() failed")
	return j
}

func Null() Builder                          { return Builder{v: nil} }
func String(s string) Builder                { return Builder{v: s} }
func Bool(x bool) Builder                    { return Builder{v: x} }
func Number(n json.Number) Builder           { return Builder{v: n} }
func RawMessage(msg json.RawMessage) Builder { return Builder{v: msg} }

func Array(elements ...Builder) Builder {
	a := make([]any, len(elements))
	for i, e := range elements {
		a[i] = e.v
	}
	return Builder{v: a}
}

func Object(elements map[string]Builder) Builder {
	a := make(map[string]any, len(elements))
	for k, e := range elements {
		a[k] = e.v
	}
	return Builder{v: a}
}
