package crosstests

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

// When using rapid.Draw is used to pull a value it calls GoString and logs the result, which is the primary way to
// interact with the printout, so the code opts to implement this. The printed values can be copied to make tests of
// their own that are not rapid-driven.
func (s prettyValueWrapper) GoString() string {
	tp := newPrettyPrinterForTypes(s.inner)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\n")

	for _, oT := range tp.DeclaredObjectTypes() {
		fmt.Fprintf(&buf, "%s := ", tp.TypeLiteral(oT))
		tp.ObjectTypeDefinition(&buf, oT)
		fmt.Fprintf(&buf, "\n")
	}

	var walk func(level int, v tftypes.Value)

	walk = func(level int, v tftypes.Value) {
		tL := tp.TypeReferenceString(v.Type())
		indent := strings.Repeat("\t", level+1)
		switch {
		case v.Type().Is(tftypes.Object{}):
			fmt.Fprintf(&buf, `tftypes.NewValue(%s, map[string]tftypes.Value{`, tL)
			var elements map[string]tftypes.Value
			err := v.As(&elements)
			contract.AssertNoErrorf(err, "this cast should always succeed")
			keys := []string{}
			for k := range elements {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&buf, "\n%s\t%q: ", indent, k)
				walk(level+1, elements[k])
				fmt.Fprintf(&buf, ",")
			}
			fmt.Fprintf(&buf, "\n%s})", indent)
		case v.Type().Is(tftypes.List{}):
			fmt.Fprintf(&buf, `tftypes.NewValue(%s, []tftypes.Value{`, tL)
			var els []tftypes.Value
			err := v.As(&els)
			contract.AssertNoErrorf(err, "this cast should always succeed")
			for _, el := range els {
				fmt.Fprintf(&buf, "\n\t%s", indent)
				walk(level+1, el)
				fmt.Fprintf(&buf, ",")
			}
			fmt.Fprintf(&buf, "\n%s})", indent)
		case v.Type().Is(tftypes.Number):
			var n big.Float
			err := v.As(&n)
			contract.AssertNoErrorf(err, "this cast should always succeed")
			fmt.Fprintf(&buf, "tftypes.NewValue(tftypes.Number, %v)", n.String())
		case v.Type().Is(tftypes.Bool):
			var b bool
			err := v.As(&b)
			contract.AssertNoErrorf(err, "this cast should always succeed")
			fmt.Fprintf(&buf, "tftypes.NewValue(tftypes.Bool, %v)", b)
		default:
			panic(fmt.Sprintf("not supported yet: %v", v.Type().String()))
		}
	}

	walk(0, s.inner)

	return buf.String()
}

type prettyPrinterForTypes struct {
	objectTypes []tftypes.Object
}

func newPrettyPrinterForTypes(v tftypes.Value) prettyPrinterForTypes {
	objectTypes := []tftypes.Object{}

	addObjectType := func(t tftypes.Type) {
		oT, ok := t.(tftypes.Object)
		if !ok {
			return
		}
		for _, alt := range objectTypes {
			if alt.Equal(oT) {
				return
			}
		}
		objectTypes = append(objectTypes, oT)
	}

	_ = tftypes.Walk(v, func(ap *tftypes.AttributePath, v tftypes.Value) (bool, error) {
		addObjectType(v.Type())
		return true, nil
	})

	return prettyPrinterForTypes{objectTypes: objectTypes}
}

func (pp prettyPrinterForTypes) DeclaredObjectTypes() []tftypes.Object {
	copy := slices.Clone(pp.objectTypes)
	slices.Reverse(copy)
	return copy
}

func (pp prettyPrinterForTypes) TypeLiteral(t tftypes.Object) string {
	for i, alt := range pp.objectTypes {
		if alt.Equal(t) {
			return fmt.Sprintf("t%d", i)
		}
	}
	contract.Failf("improper use of the type pretty-printer")
	return ""
}

func (pp prettyPrinterForTypes) ObjectTypeDefinition(w io.Writer, ty tftypes.Object) {
	fmt.Fprintf(w, "tftypes.Object{AttributeTypes: map[string]tftypes.Type{")
	keys := []string{}
	for k := range ty.AttributeTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := ty.AttributeTypes[k]
		fmt.Fprintf(w, "\n\t")
		fmt.Fprintf(w, "\t%q: ", k)
		pp.TypeReference(w, t)
		fmt.Fprintf(w, ",")
	}
	fmt.Fprintf(w, "\n}}")
}

func (pp prettyPrinterForTypes) TypeReferenceString(t tftypes.Type) string {
	var buf bytes.Buffer
	pp.TypeReference(&buf, t)
	return buf.String()
}

func (pp prettyPrinterForTypes) TypeReference(w io.Writer, t tftypes.Type) {
	switch {
	case t.Is(tftypes.Object{}):
		fmt.Fprintf(w, pp.TypeLiteral(t.(tftypes.Object)))
	case t.Is(tftypes.List{}):
		fmt.Fprintf(w, "tftypes.List{ElementType: ")
		pp.TypeReference(w, t.(tftypes.List).ElementType)
		fmt.Fprintf(w, "}")
	case t.Is(tftypes.Set{}):
		fmt.Fprintf(w, "tftypes.Set{ElementType: ")
		pp.TypeReference(w, t.(tftypes.Set).ElementType)
		fmt.Fprintf(w, "}")
	case t.Is(tftypes.Map{}):
		fmt.Fprintf(w, "tftypes.Map{ElementType: ")
		pp.TypeReference(w, t.(tftypes.Map).ElementType)
		fmt.Fprintf(w, "}")
	case t.Is(tftypes.String):
		fmt.Fprintf(w, "tftypes.String")
	case t.Is(tftypes.Number):
		fmt.Fprintf(w, "tftypes.Number")
	case t.Is(tftypes.Bool):
		fmt.Fprintf(w, "tftypes.Bool")
	default:
		contract.Failf("Not supported yet")
	}
}
