package pfutils

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
)

// These interfaces are re-implemented here from "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
// as we can not link to them directly.

type attributeLikeWithBoolDefaultValue interface {
	AttrLike
	BoolDefaultValue() defaults.Bool
}

type attributeLikeWithFloat32DefaultValue interface {
	AttrLike
	Float32DefaultValue() defaults.Float32
}

type attributeLikeWithFloat64DefaultValue interface {
	AttrLike
	Float64DefaultValue() defaults.Float64
}

type attributeLikeWithInt32DefaultValue interface {
	AttrLike
	Int32DefaultValue() defaults.Int32
}

type attributeLikeWithInt64DefaultValue interface {
	AttrLike
	Int64DefaultValue() defaults.Int64
}

type attributeLikeWithListDefaultValue interface {
	AttrLike
	ListDefaultValue() defaults.List
}

type attributeLikeWithMapDefaultValue interface {
	AttrLike
	MapDefaultValue() defaults.Map
}

type attributeLikeWithNumberDefaultValue interface {
	AttrLike
	NumberDefaultValue() defaults.Number
}

type attributeLikeWithObjectDefaultValue interface {
	AttrLike
	ObjectDefaultValue() defaults.Object
}

type attributeLikeWithSetDefaultValue interface {
	AttrLike
	SetDefaultValue() defaults.Set
}

type attributeLikeWithStringDefaultValue interface {
	AttrLike
	StringDefaultValue() defaults.String
}

type attributeLikeWithDynamicDefaultValue interface {
	AttrLike
	DynamicDefaultValue() defaults.Dynamic
}

func hasDefault(attr AttrLike) bool {
	switch a := attr.(type) {
	case attributeLikeWithBoolDefaultValue:
		return a.BoolDefaultValue() != nil
	case attributeLikeWithFloat32DefaultValue:
		return a.Float32DefaultValue() != nil
	case attributeLikeWithFloat64DefaultValue:
		return a.Float64DefaultValue() != nil
	case attributeLikeWithInt32DefaultValue:
		return a.Int32DefaultValue() != nil
	case attributeLikeWithInt64DefaultValue:
		return a.Int64DefaultValue() != nil
	case attributeLikeWithListDefaultValue:
		return a.ListDefaultValue() != nil
	case attributeLikeWithMapDefaultValue:
		return a.MapDefaultValue() != nil
	case attributeLikeWithNumberDefaultValue:
		return a.NumberDefaultValue() != nil
	case attributeLikeWithObjectDefaultValue:
		return a.ObjectDefaultValue() != nil
	case attributeLikeWithSetDefaultValue:
		return a.SetDefaultValue() != nil
	case attributeLikeWithStringDefaultValue:
		return a.StringDefaultValue() != nil
	case attributeLikeWithDynamicDefaultValue:
		return a.DynamicDefaultValue() != nil
	default:
		return false
	}
}
