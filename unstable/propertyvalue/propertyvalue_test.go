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

	"pgregory.net/rapid"

	rtesting "github.com/pulumi/pulumi/sdk/v3/go/common/resource/testing"
)

func TestRemoveSecrets(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		randomPV := rtesting.PropertyValueGenerator(5 /* maxDepth */).Draw(t, "pv")
		if RemoveSecrets(randomPV).ContainsSecrets() {
			t.Fatalf("RemoveSecrets(randomPV).ContainsSecrets()")
		}
	})
}
