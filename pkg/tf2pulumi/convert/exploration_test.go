// Copyright 2016-2021, Pulumi Corporation.
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

package convert

import (
	"github.com/zclconf/go-cty/cty"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"pgregory.net/rapid"
)

func TestAnton(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		exampleType := typeGen(3).Draw(t, "exampleType").(model.Type)

		err := checkTypeTurnaround(exampleType)
		if err != nil {
			t.Error(err)
		}
	})
}

func typeGen(depth int) *rapid.Generator {
	leafGen := rapid.OneOf(
		// noneTypeGen(),
		opaqueTypeGen(),
		//constTypeGen(),
	)

	if depth <= 0 {
		return leafGen
	}

	typeGen1 := typeGen(depth - 1)

	return rapid.OneOf(
		leafGen,
		listTypeGen(typeGen1),
		mapTypeGen(typeGen1),
		objectTypeGen(typeGen1),
		//outputTypeGen(typeGen1),
		//promiseTypeGen(typeGen1),
		setTypeGen(typeGen1),
		tupleTypeGen(typeGen1),
		unionTypeGen(typeGen1),
	)
}

func noneTypeGen() *rapid.Generator {
	return rapid.Just(model.NoneType)
}

func opaqueTypeGen() *rapid.Generator {
	customGen := rapid.Custom(func(t *rapid.T) model.Type {
		for {
			typeName := rapid.String().Draw(t, "typeName").(string)
			t, err := model.NewOpaqueType(typeName) // NOTE: no annotations
			if err == nil {
				return t
			}
		}
	})
	return rapid.OneOf(
		rapid.Just(model.BoolType),
		rapid.Just(model.IntType),
		rapid.Just(model.NumberType),
		rapid.Just(model.StringType),
		//rapid.Just(model.DynamicType),
		customGen,
	)
}

func constTypeGen() *rapid.Generator {
	return rapid.OneOf(
		rapid.Just(model.NewConstType(model.BoolType, cty.BoolVal(true))),
		rapid.Just(model.NewConstType(model.BoolType, cty.BoolVal(false))),
		rapid.Custom(func(t *rapid.T) model.Type {
			i := rapid.Int64().Draw(t, "i").(int64)
			return model.NewConstType(model.IntType, cty.NumberIntVal(i))
		}),
		rapid.Custom(func(t *rapid.T) model.Type {
			i := rapid.Float64().Draw(t, "f").(float64)
			return model.NewConstType(model.NumberType, cty.NumberFloatVal(i))
		}),
		rapid.Custom(func(t *rapid.T) model.Type {
			s := rapid.String().Draw(t, "s").(string)
			return model.NewConstType(model.StringType, cty.StringVal(s))
		}),
	)
}

func listTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) model.Type {
		listTypeArg := typeGen.Draw(t, "listTypeArg").(model.Type)
		return model.NewListType(listTypeArg)
	})
}

func mapTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) model.Type {
		mapTypeArg := typeGen.Draw(t, "mapTypeArg").(model.Type)
		return model.NewMapType(mapTypeArg)
	})
}

func fieldNameGen() *rapid.Generator {
	return rapid.StringMatching("[a-z]{1,3}")
}

func objectTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	objPropsGen := rapid.MapOfN(fieldNameGen(), typeGen, 0, 3)
	return rapid.Custom(func(t *rapid.T) model.Type {
		props := objPropsGen.Draw(t, "props").(map[string]interface{})
		propsMap := map[string]model.Type{}
		for k, v := range props {
			propsMap[k] = v.(model.Type)
		}
		return model.NewObjectType(propsMap) // NOTE: no annotations for now
	})
}

func outputTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) model.Type {
		outputTypeArg := typeGen.Draw(t, "outputTypeArg").(model.Type)
		return model.NewOutputType(outputTypeArg)
	})
}

func promiseTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) model.Type {
		promiseTypeArg := typeGen.Draw(t, "promiseTypeArg").(model.Type)
		return model.NewPromiseType(promiseTypeArg)
	})
}

func setTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) model.Type {
		setTypeArg := typeGen.Draw(t, "setTypeArg").(model.Type)
		return model.NewSetType(setTypeArg)
	})
}

func tupleTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	argsGen := typeSliceGen(typeGen, 2, 3) // TODO what's the lower bound?
	return rapid.Custom(func(t *rapid.T) model.Type {
		args := argsGen.Draw(t, "args").([]model.Type)
		return model.NewTupleType(args...)
	})
}

func typeSliceGen(typeGen *rapid.Generator, minLen, maxLen int) *rapid.Generator {
	argsGen := rapid.SliceOfN(typeGen, minLen, maxLen)
	return rapid.Custom(func(t *rapid.T) []model.Type {
		args := argsGen.Draw(t, "args").([]interface{})
		var typeArgs []model.Type
		for _, arg := range args {
			typeArgs = append(typeArgs, arg.(model.Type))
		}
		return typeArgs
	})
}

func unionTypeGen(typeGen *rapid.Generator) *rapid.Generator {
	argsGen := rapid.SliceOfN(typeGen, 1, 3) // TODO what's the lower bound?
	return rapid.Custom(func(t *rapid.T) model.Type {
		args := argsGen.Draw(t, "args").([]interface{})
		var typeArgs []model.Type
		for _, arg := range args {
			typeArgs = append(typeArgs, arg.(model.Type))
		}
		return model.NewUnionType(typeArgs...)
	})
}
