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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestKeySuggestions(t *testing.T) {
	schemaMap := &schema.SchemaMap{
		"my_prop": (&schema.Schema{
			Type:     shim.TypeString,
			Optional: true,
		}).Shim(),
	}

	assert.Empty(t, keySuggestions(NewCheckFailurePath(schemaMap, nil, "my_prop"), schemaMap, nil))
	assert.Contains(t, keySuggestions(NewCheckFailurePath(schemaMap, nil, "myprop"), schemaMap, nil),
		resource.PropertyKey("myProp"))
}
