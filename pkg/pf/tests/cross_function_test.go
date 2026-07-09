// Copyright 2016-2026, Pulumi Corporation.
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

package tfbridgetests

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

// A provider-defined function must produce the same value whether Terraform calls it
// (provider::testbridge::fn(...)) or the bridged Pulumi provider invokes it. Non-object
// returns arrive under the bridge's "result" wrapper property on the Pulumi side, so
// scalar cases compare against that wrapping.
func TestCrossFunctionParity(t *testing.T) {
	t.Parallel()
	provider := func() *pb.Provider {
		return pb.NewProvider(pb.NewProviderArgs{
			TypeName:     "testbridge",
			AllFunctions: testprovider.SyntheticTestBridgeFunctions(),
		})
	}

	t.Run("variadic string result", func(t *testing.T) {
		t.Parallel()
		res := crosstests.Function(t, provider(),
			`provider::testbridge::concat("-", "a", "b", "c")`,
			"testbridge:index/concat:concat",
			resource.PropertyMap{
				"separator": resource.NewStringProperty("-"),
				"parts": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("a"),
					resource.NewStringProperty("b"),
					resource.NewStringProperty("c"),
				}),
			})
		assert.Equal(t, map[string]any{"result": res.TF}, res.Pulumi)
	})

	t.Run("variadic with no trailing arguments", func(t *testing.T) {
		t.Parallel()
		res := crosstests.Function(t, provider(),
			`provider::testbridge::concat("-")`,
			"testbridge:index/concat:concat",
			resource.PropertyMap{
				"separator": resource.NewStringProperty("-"),
			})
		assert.Equal(t, map[string]any{"result": res.TF}, res.Pulumi)
	})

	t.Run("object result", func(t *testing.T) {
		t.Parallel()
		res := crosstests.Function(t, provider(),
			`provider::testbridge::parse_id("foo/bar")`,
			"testbridge:index/parseId:parseId",
			resource.PropertyMap{
				"id": resource.NewStringProperty("foo/bar"),
			})
		assert.Equal(t, res.TF, any(res.Pulumi))
	})

	t.Run("null argument maps to omitted argument", func(t *testing.T) {
		t.Parallel()
		res := crosstests.Function(t, provider(),
			`provider::testbridge::nullable_default(null)`,
			"testbridge:index/nullableDefault:nullableDefault",
			resource.PropertyMap{})
		assert.Equal(t, map[string]any{"result": res.TF}, res.Pulumi)
	})

	t.Run("nullable argument provided", func(t *testing.T) {
		t.Parallel()
		res := crosstests.Function(t, provider(),
			`provider::testbridge::nullable_default("explicit")`,
			"testbridge:index/nullableDefault:nullableDefault",
			resource.PropertyMap{
				"value": resource.NewStringProperty("explicit"),
			})
		assert.Equal(t, map[string]any{"result": res.TF}, res.Pulumi)
	})
}
