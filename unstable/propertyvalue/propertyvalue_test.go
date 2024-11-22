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

package propertyvalue

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	rtesting "github.com/pulumi/pulumi/sdk/v3/go/common/resource/testing"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestRemoveSecrets(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		randomPV := rtesting.PropertyValueGenerator(5 /* maxDepth */).Draw(t, "pv")
		if RemoveSecrets(randomPV).ContainsSecrets() {
			t.Fatalf("RemoveSecrets(randomPV).ContainsSecrets()")
		}
	})
}

func TestRemoveSecretsAndOutputs(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		randomPV := rtesting.PropertyValueGenerator(5 /* maxDepth */).Draw(t, "pv")
		result := RemoveSecretsAndOutputs(randomPV)
		if result.ContainsSecrets() {
			t.Fatalf("RemoveSecretsAndOutputs(randomPV).ContainsSecrets()")
		}

		visitor := func(path resource.PropertyPath, val resource.PropertyValue) (resource.PropertyValue, error) {
			require.False(t, val.IsSecret())
			require.False(t, val.IsOutput())
			return val, nil
		}

		_, err := TransformPropertyValue(resource.PropertyPath{}, visitor, result)
		require.NoError(t, err)
	})
}

func TestIsNilArray(t *testing.T) {
	t.Parallel()

	require.True(t, isNilArray(resource.PropertyValue{V: []resource.PropertyValue(nil)}))
	require.False(t, isNilArray(resource.PropertyValue{V: []resource.PropertyValue{}}))
}

func TestIsNilObject(t *testing.T) {
	t.Parallel()

	require.True(t, isNilObject(resource.PropertyValue{V: resource.PropertyMap(nil)}))
	require.False(t, isNilObject(resource.PropertyValue{V: resource.PropertyMap{}}))
}

func TestTransformPreservesNilArrays(t *testing.T) {
	t.Parallel()

	nilArrayPV := resource.PropertyValue{V: []resource.PropertyValue(nil)}
	result := Transform(func(value resource.PropertyValue) resource.PropertyValue {
		return value
	}, nilArrayPV)

	require.True(t, result.IsArray())
	require.Nil(t, result.ArrayValue())
}

func TestTransformPreservesNilObjects(t *testing.T) {
	t.Parallel()

	nilObjectPV := resource.PropertyValue{V: resource.PropertyMap(nil)}
	result := Transform(func(value resource.PropertyValue) resource.PropertyValue {
		return value
	}, nilObjectPV)
	require.True(t, result.IsObject())
	require.Nil(t, result.ObjectValue())
}
