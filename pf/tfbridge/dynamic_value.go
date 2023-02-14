// Copyright 2016-2023, Pulumi Corporation.
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

package tfbridge

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func makeDynamicValue(a tftypes.Value) (tfprotov6.DynamicValue, error) {
	var n tfprotov6.DynamicValue
	av, err := tfprotov6.NewDynamicValue(a.Type(), a)
	if err != nil {
		return n, err
	}
	return av, nil
}

func makeDynamicValues2(a, b tftypes.Value) (tfprotov6.DynamicValue, tfprotov6.DynamicValue, error) {
	var n tfprotov6.DynamicValue
	av, err := tfprotov6.NewDynamicValue(a.Type(), a)
	if err != nil {
		return n, n, err
	}
	bv, err := tfprotov6.NewDynamicValue(b.Type(), b)
	if err != nil {
		return n, n, err
	}
	return av, bv, nil
}

func makeDynamicValues3(a, b, c tftypes.Value) (
	tfprotov6.DynamicValue, tfprotov6.DynamicValue,
	tfprotov6.DynamicValue,
	error,
) {
	var n tfprotov6.DynamicValue
	av, err := tfprotov6.NewDynamicValue(a.Type(), a)
	if err != nil {
		return n, n, n, err
	}
	bv, err := tfprotov6.NewDynamicValue(b.Type(), b)
	if err != nil {
		return n, n, n, err
	}
	cv, err := tfprotov6.NewDynamicValue(c.Type(), c)
	if err != nil {
		return n, n, n, err
	}
	return av, bv, cv, nil
}
