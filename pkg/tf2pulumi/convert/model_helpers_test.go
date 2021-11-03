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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

func TestSetConfigBlockType(t *testing.T) {
	check := func(name string, ty model.Type, expectError bool) {
		t.Run(name, func(t *testing.T) {
			block := &model.Block{Type: "config", Labels: []string{"x"}}
			err := setConfigBlockType(block, ty)
			if expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, fmt.Sprintf("%v", ty), block.Labels[1])
			}
		})
	}

	checkOk := func(name string, ty model.Type) {
		check(name, ty, false)
	}

	checkError := func(name string, ty model.Type) {
		check(name, ty, true)
	}

	checkOk("intType", model.IntType)
	checkOk("boolType", model.BoolType)
	checkOk("objType", model.NewObjectType(map[string]model.Type{"foo": model.IntType}))

	// below we expect errors - this may improve in the future and
	// stop failing; including to demonstrate the reason for why
	// `setConfigBlockType` exists

	constType := model.NewConstType(model.IntType, cty.NumberIntVal(1))
	checkError("constType", constType)

	objTypeWithConst := model.NewObjectType(map[string]model.Type{"foo": constType})
	checkError("objectTypeWithConstType", objTypeWithConst)
}

func TestGeneralizeConstType(t *testing.T) {
	constType := model.NewConstType(model.IntType, cty.NumberIntVal(1))
	assert.True(t, generalizeConstType(constType).Equals(model.IntType))

	objType := model.NewObjectType(map[string]model.Type{"foo": model.IntType})
	objTypeWithConst := model.NewObjectType(map[string]model.Type{"foo": constType})
	assert.True(t, generalizeConstType(objTypeWithConst).Equals(objType))
}
