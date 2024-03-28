package crosstests

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type quickBuilder struct {
}

func qb() *quickBuilder {
	return &quickBuilder{}
}

func (q *quickBuilder) objT() quickType {
	return quickType{inner: tftypes.Object{}}
}

func (q *quickBuilder) strT() quickType {
	return quickType{inner: tftypes.String}
}

func (q *quickBuilder) str(x string) quickValue {
	return func(tftypes.Type) tftypes.Value {
		return tftypes.NewValue(tftypes.String, x)
	}
}

func (q *quickBuilder) unk() quickValue {
	return func(t tftypes.Type) tftypes.Value {
		return tftypes.NewValue(t, tftypes.UnknownValue)
	}
}

func (q *quickBuilder) obj() quickValue {
	return func(t tftypes.Type) tftypes.Value {
		contract.Assertf(t.Is(tftypes.Object{}), "expected object type, got %s", t)
		m := map[string]tftypes.Value{}
		for k, v := range t.(tftypes.Object).AttributeTypes {
			m[k] = q.null().build(quickType{v})
		}
		return tftypes.NewValue(t, m)
	}
}

func (q *quickBuilder) null() quickValue {
	return func(t tftypes.Type) tftypes.Value {
		return tftypes.NewValue(t, nil)
	}
}

type quickType struct {
	inner tftypes.Type
}

func (qt quickType) fld(name string, t quickType) quickType {
	contract.Assertf(qt.inner.Is(tftypes.Object{}), "expected object type, got %s", qt.inner)
	copy := map[string]tftypes.Type{}
	for k, v := range qt.inner.(tftypes.Object).AttributeTypes {
		copy[k] = v
	}
	copy[name] = t.inner
	return quickType{tftypes.Object{
		AttributeTypes: copy,
	}}
}

type quickValue func(tftypes.Type) tftypes.Value

func (qv quickValue) build(t quickType) tftypes.Value {
	return qv(t.inner)
}

func (qv quickValue) fld(name string, value quickValue) quickValue {
	return func(t tftypes.Type) tftypes.Value {
		contract.Assertf(t.Is(tftypes.Object{}), "expected object type, got %s", t)
		attrTy := t.(tftypes.Object).AttributeTypes[name]
		contract.Assertf(attrTy != nil, "cannot find attribute type for %q", name)
		old := qv(t)
		dst := map[string]tftypes.Value{}
		err := old.As(&dst)
		contract.Assertf(err == nil, "expected object value, got %s", old)
		dst[name] = value(attrTy)
		return tftypes.NewValue(t, dst)
	}
}
