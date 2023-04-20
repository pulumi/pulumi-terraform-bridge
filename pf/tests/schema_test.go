// Copyright 2016-2022, Pulumi Corporation.
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
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
)

func TestSchemaGen(t *testing.T) {
	t.Run("random", func(t *testing.T) {
		genMetadata(t, testprovider.RandomProvider())
	})
	t.Run("tls", func(t *testing.T) {
		genMetadata(t, testprovider.TLSProvider())
	})
	t.Run("testbridge", func(t *testing.T) {
		data := genMetadata(t, testprovider.SyntheticTestBridgeProvider())
		var spec schema.PackageSpec
		require.NoError(t, json.Unmarshal(data.PackageSchema, &spec))
		res := spec.Resources["testbridge:index/testnest:Testnest"]
		assert.Equal(t, "array", res.InputProperties["rules"].Type)
		assert.Equal(t,
			"#/types/testbridge:index/TestnestRule:TestnestRule",
			res.InputProperties["rules"].Items.Ref)

		rule := spec.Types["testbridge:index/TestnestRule:TestnestRule"]
		assert.Equal(t,
			"#/types/testbridge:index/TestnestRuleActionParameters:TestnestRuleActionParameters",
			rule.Properties["actionParameters"].Ref)
		assert.Equal(t, "", rule.Properties["actionParameters"].Type)
	})
}
