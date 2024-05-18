// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Define pretty-printers to make test output easier to interpret.
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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/valast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Write out schema values using [valast.String]. This may break down once we start testing callbacks, but works for
// simple schemas and makes it easier to read test printout.
type prettySchemaWrapper struct {
	sch schema.Schema
}

func (psw prettySchemaWrapper) GoString() string {
	return valast.String(psw.sch)
}

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
		indent := strings.Repeat("  ", level)
		switch {
		case v.IsNull():
			// TODO
			fmt.Fprintf(&buf, "tftypes.NewValue(%s, nil)", tL)
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
				fmt.Fprintf(&buf, "\n%s  %q: ", indent, k)
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
				fmt.Fprintf(&buf, "\n. %s", indent)
				walk(level+1, el)
				fmt.Fprintf(&buf, ",")
			}
			fmt.Fprintf(&buf, "\n%s})", indent)
		case v.Type().Is(tftypes.Set{}):
			fmt.Fprintf(&buf, `tftypes.NewValue(%s, []tftypes.Value{`, tL)
			var els []tftypes.Value
			err := v.As(&els)
			contract.AssertNoErrorf(err, "this cast should always succeed")
			for _, el := range els {
				fmt.Fprintf(&buf, "\n. %s", indent)
				walk(level+1, el)
				fmt.Fprintf(&buf, ",")
			}
			fmt.Fprintf(&buf, "\n%s})", indent)
		case v.Type().Is(tftypes.Map{}):
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
				fmt.Fprintf(&buf, "\n%s  %q: ", indent, k)
				walk(level+1, elements[k])
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
		case v.Type().Is(tftypes.String):
			var s string
			err := v.As(&s)
			contract.AssertNoErrorf(err, "this cast should always succeed")
			fmt.Fprintf(&buf, "tftypes.NewValue(tftypes.String, %q)", s)
		default:
			panic(fmt.Sprintf("not supported yet: %v", v.Type().String()))
		}
	}

	walk(0, s.inner)

	return buf.String()
}

// Assist [prettyValueWrapper] to write out types nicely.
type prettyPrinterForTypes struct {
	objectTypes []tftypes.Object
}

func newPrettyPrinterForTypes(v tftypes.Value) prettyPrinterForTypes {
	objectTypes := []tftypes.Object{}

	var visitTypes func(t tftypes.Type, vis func(tftypes.Type))
	visitTypes = func(t tftypes.Type, vis func(tftypes.Type)) {
		vis(t)
		switch {
		case t.Is(tftypes.Object{}):
			for _, v := range t.(tftypes.Object).AttributeTypes {
				visitTypes(v, vis)
			}
		case t.Is(tftypes.List{}):
			visitTypes(t.(tftypes.List).ElementType, vis)
		case t.Is(tftypes.Map{}):
			visitTypes(t.(tftypes.Map).ElementType, vis)
		case t.Is(tftypes.Set{}):
			visitTypes(t.(tftypes.Set).ElementType, vis)
		case t.Is(tftypes.Tuple{}):
			for _, et := range t.(tftypes.Tuple).ElementTypes {
				visitTypes(et, vis)
			}
		default:
			return
		}
	}

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
		visitTypes(v.Type(), addObjectType)
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
	contract.Failf("improper use of the type pretty-printer: %v", t.String())
	return ""
}

func (pp prettyPrinterForTypes) ObjectTypeDefinition(w io.Writer, ty tftypes.Object) {
	if len(ty.AttributeTypes) == 0 {
		fmt.Fprintf(w, "tftypes.Object{}")
		return
	}
	fmt.Fprintf(w, "tftypes.Object{AttributeTypes: map[string]tftypes.Type{")
	keys := []string{}
	for k := range ty.AttributeTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := ty.AttributeTypes[k]
		fmt.Fprintf(w, "\n  %q: ", k)
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
		fmt.Fprintf(w, "%s", pp.TypeLiteral(t.(tftypes.Object)))
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
