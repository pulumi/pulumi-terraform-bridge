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

// This module provides helper functions to work with
// `github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model` that have not
// yet made it upstream.

package convert

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	//"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// Helps assign a type to a `Type="config"` block. It is currently not
// apparent from the types, but blocks carry 1 or 2 labels, with the
// second optional label encoding the type of the variable.
//
// Example block in HCL syntax:
//
//	config foo "int" {
//	   default = 1
//	}
//
// The type is decoded downstream (as in
// `github.com/pulumi/pulumi/pkg/codegen/pcl/binder.go`) in PCL
// processing. Currently we cannot guarantee that all posible
// `model.Type` values survive the string encoding and subsequent
// decoding. Thereore this function mimicks downstream decoding to see
// if the value turns around, and aborts processing if that is not the
// case.
func setConfigBlockType(block *model.Block, variableType model.Type) error {
	if block.Type != "config" {
		return fmt.Errorf("setConfigBlockType should only be used on block.Type='config' blocks")
	}

	if len(block.Labels) >= 2 {
		return fmt.Errorf("setConfigBlockType refuses to overwrite block.Label[1]")
	}

	tempScope := model.TypeScope.Push(nil)
	typeString := fmt.Sprintf("%v", variableType)
	typeExpr, diags := model.BindExpressionText(
		typeString,
		tempScope,
		hcl.Pos{})

	if typeExpr == nil || diags.HasErrors() {
		return fmt.Errorf(
			"Type %s prints as '%s' but cannot be parsed back. Diagnostics: %v",
			variableType.String(),
			typeString,
			diags)
	}

	if variableType.String() != typeExpr.Type().String() {
		return fmt.Errorf(
			"Type T1=%s prints as '%s' which parses as T2=%s, expected T1=T2",
			variableType.String(),
			typeString,
			typeExpr.Type(),
		)
	}

	block.Labels = append(block.Labels, typeString)
	return nil
}

// Substitutes `model.ConstType` with the underlying type.
func generalizeConstType(t model.Type) model.Type {
	return typeTransform(t, func(t model.Type) model.Type {
		switch sT := t.(type) {
		case *model.ConstType:
			return sT.Type
		default:
			return t
		}
	})
}

// Views `model.Type` as a recursive tree and transforms a given tree
// `t` by wrapping the `transform` around every node.
func typeTransform(t model.Type, transform func(model.Type) model.Type) model.Type {
	rec := func(t model.Type) model.Type {
		return typeTransform(t, transform)
	}
	recSlice := func(ts []model.Type) []model.Type {
		var result []model.Type
		for _, t := range ts {
			result = append(result, rec(t))
		}
		return result
	}
	recMap := func(ts map[string]model.Type) map[string]model.Type {
		result := map[string]model.Type{}
		for k, t := range ts {
			result[k] = rec(t)
		}
		return result
	}
	var t2 model.Type
	switch sT := t.(type) {
	case *model.ConstType:
		t2 = model.NewConstType(rec(sT.Type), sT.Value)
	case *model.ListType:
		t2 = model.NewListType(rec(sT.ElementType))
	case *model.MapType:
		t2 = model.NewMapType(rec(sT.ElementType))
	case *model.ObjectType:
		t2 = model.NewObjectType(recMap(sT.Properties), sT.Annotations...)
	case *model.OutputType:
		t2 = model.NewOutputType(rec(sT.ElementType))
	case *model.PromiseType:
		t2 = model.NewPromiseType(rec(sT.ElementType))
	case *model.SetType:
		t2 = model.NewSetType(rec(sT.ElementType))
	case *model.TupleType:
		t2 = model.NewTupleType(recSlice(sT.ElementTypes)...)
	case *model.UnionType:
		t2 = model.NewUnionType(recSlice(sT.ElementTypes)...)
	default:
		t2 = t
	}
	return transform(t2)
}
