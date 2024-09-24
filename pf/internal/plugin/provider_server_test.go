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

package plugin

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	prapid "github.com/pulumi/pulumi/sdk/v3/go/property/testing"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// TestRemoveSecrets checks that removeSecrets removes [resource.Secret] values and unsets
// [resource.Output.Secret] fields without making any other changes.
func TestRemoveSecrets(t *testing.T) {

	// These functions validate that a diff does not contain any non-secret changes.
	var (
		validateObjectDiff func(assert.TestingT, resource.ObjectDiff)
		validateArrayDiff  func(assert.TestingT, resource.ArrayDiff)
		validateValueDiff  func(assert.TestingT, resource.ValueDiff)
	)

	validateValueDiff = func(t assert.TestingT, v resource.ValueDiff) {
		switch {
		case v.Old.IsOutput():
			oOld := v.Old.OutputValue()
			oOld.Secret = false
			if d := resource.NewProperty(oOld).DiffIncludeUnknowns(v.New); d != nil {
				validateValueDiff(t, *d)
			}
		case v.Old.IsSecret():
			if d := v.Old.SecretValue().Element.DiffIncludeUnknowns(v.New); d != nil {
				validateValueDiff(t, *d)
			}
		case v.Old.IsObject():
			validateObjectDiff(t, *v.Object)
		case v.Old.IsArray():
			validateArrayDiff(t, *v.Array)
		default:
			assert.Failf(t, "", "unexpected Update.Old type %q", v.Old.TypeString())
		}
	}

	validateArrayDiff = func(t assert.TestingT, diff resource.ArrayDiff) {
		assert.Empty(t, diff.Adds)
		assert.Empty(t, diff.Deletes)

		for _, v := range diff.Updates {
			validateValueDiff(t, v)
		}
	}

	validateObjectDiff = func(t assert.TestingT, diff resource.ObjectDiff) {
		assert.Empty(t, diff.Adds)

		// Diff does not distinguish from a missing key and a null property, so
		// when we go from a map{k: secret(null)} to a map{k: null}, the diff
		// machinery shows a delete.
		//
		// We have an explicit test for this behavior.
		for _, v := range diff.Deletes {
			assert.Equal(t, resource.MakeSecret(resource.NewNullProperty()), v)
		}

		for _, v := range diff.Updates {
			validateValueDiff(t, v)
		}
	}

	t.Run("rapid", rapid.MakeCheck(func(t *rapid.T) {
		m := resource.ToResourcePropertyValue(prapid.Map(5).Draw(t, "top-level")).ObjectValue()
		if m.ContainsSecrets() {
			unsecreted := removeSecrets(m)
			assert.False(t, unsecreted.ContainsSecrets())

			// We need to assert that the only change between m and unsecreted
			// is that secret values went to their element values.
			if d := m.DiffIncludeUnknowns(unsecreted); d != nil {
				validateObjectDiff(t, *d)
			}
		} else {
			assert.Equal(t, m, removeSecrets(m))
		}
	}))

	t.Run("map-null-secrets", func(t *testing.T) {
		assert.Equal(t,
			resource.PropertyMap{
				"null": resource.NewNullProperty(),
			},
			removeSecrets(resource.PropertyMap{
				"null": resource.MakeSecret(resource.NewNullProperty()),
			}),
		)
	})
}
