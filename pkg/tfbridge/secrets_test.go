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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestMarkSchemaSecrets(t *testing.T) {
	type testCase struct {
		name   string
		pv     resource.PropertyValue
		expect resource.PropertyValue
	}

	schemaMap := schema.SchemaMap{
		"simple_string_prop": (&schema.Schema{
			Type:     shim.TypeString,
			Optional: true,
		}).Shim(),
		"unsecret_string_prop": (&schema.Schema{
			Type:      shim.TypeString,
			Sensitive: true,
			Optional:  true,
		}).Shim(),
		"sensitive_string_prop": (&schema.Schema{
			Type:      shim.TypeString,
			Optional:  true,
			Sensitive: true,
		}).Shim(),
		"list_with_sensitive_elem": (&schema.Schema{
			Type:     shim.TypeList,
			Optional: true,
			Elem: (&schema.Schema{
				Type:      shim.TypeString,
				Optional:  true,
				Sensitive: true,
			}).Shim(),
		}).Shim(),
	}

	yes, no := true, false

	configInfos := map[string]*SchemaInfo{
		"simple_string_prop": {
			Secret: &yes,
		},
		"unsecret_string_prop": {
			Secret: &no,
		},
	}

	obj1 := func(name string, v resource.PropertyValue) resource.PropertyValue {
		return resource.NewObjectProperty(resource.PropertyMap{
			resource.PropertyKey(name): v,
		})
	}

	arr := func(v ...resource.PropertyValue) resource.PropertyValue {
		return resource.NewArrayProperty(v)
	}

	str := resource.NewStringProperty
	sec := resource.MakeSecret

	testCases := []testCase{
		{
			"marks sensitive string prop as secret",
			obj1("sensitiveStringProp", str("secret")),
			obj1("sensitiveStringProp", sec(str("secret"))),
		},
		{
			"does not double-mark secrets",
			obj1("sensitiveStringProp", sec(str("secret"))),
			obj1("sensitiveStringProp", sec(str("secret"))),
		},
		{
			"marks sensitive list elements as secret",
			obj1("listWithSensitiveElems", arr(str("secret1"), str("secret2"))),
			obj1("listWithSensitiveElems", arr(sec(str("secret1")), sec(str("secret2")))),
		},
		{
			"respects secret overrides",
			obj1("simpleStringProp", str("secret")),
			obj1("simpleStringProp", sec(str("secret"))),
		},
		{
			"respects no-secret overrides",
			obj1("unsecretStringProp", str("secret")),
			obj1("unsecretStringProp", str("secret")),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			actual := MarkSchemaSecrets(
				context.Background(),
				schemaMap,
				configInfos,
				tc.pv)
			assert.Equal(t, tc.expect, actual)
		})
	}
}
