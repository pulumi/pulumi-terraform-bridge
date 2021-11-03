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
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// Helps assign a type to a `Type="config"` block. It is currently not
// apparent from the types, but blocks carry 1 or 2 labels, with the
// second optional label encoding the type of the variable.
//
// Example block in HCL syntax:
//
//     config foo "int" {
//        default = 1
//     }
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

	err := checkTypeTurnaround(variableType)
	if err != nil {
		return err
	}

	block.Labels = append(block.Labels, fmt.Sprintf("%v", variableType))
	return nil
}

func checkTypeTurnaround(t model.Type) error {
	tempScope := model.TypeScope.Push(nil)
	typeString := fmt.Sprintf("%v", t)
	typeExpr, diags := model.BindExpressionText(
		typeString,
		tempScope,
		hcl.Pos{})

	if typeExpr == nil || diags.HasErrors() {
		return fmt.Errorf(
			"Type %s (instance of %v) prints as '%s' but cannot be parsed back. Diagnostics: %v",
			t.String(),
			reflect.TypeOf(t),
			typeString,
			diags)
	}

	if t.String() != typeExpr.Type().String() {
		return fmt.Errorf(
			"Type T1=%s prints as '%s' which parses as T2=%s, expected T1=T2",
			t.String(),
			typeString,
			typeExpr.Type(),
		)
	}

	return nil
}
