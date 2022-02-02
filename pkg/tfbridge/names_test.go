// Copyright 2016-2018, Pulumi Corporation.
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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
)

func TestPulumiToTerraformName(t *testing.T) {
	assert.Equal(t, "", PulumiToTerraformName("", nil, nil))
	assert.Equal(t, "test", PulumiToTerraformName("test", nil, nil))
	assert.Equal(t, "test_name", PulumiToTerraformName("testName", nil, nil))
	assert.Equal(t, "test_name_pascal", PulumiToTerraformName("TestNamePascal", nil, nil))
	assert.Equal(t, "test_name", PulumiToTerraformName("test_name", nil, nil))
	assert.Equal(t, "test_name_", PulumiToTerraformName("testName_", nil, nil))
	assert.Equal(t, "t_e_s_t_n_a_m_e", PulumiToTerraformName("TESTNAME", nil, nil))
}

func TestTerraformToPulumiName(t *testing.T) {
	assert.Equal(t, "", TerraformToPulumiName("", nil, nil, false))
	assert.Equal(t, "test", TerraformToPulumiName("test", nil, nil, false))
	assert.Equal(t, "testName", TerraformToPulumiName("test_name", nil, nil, false))
	assert.Equal(t, "testName_", TerraformToPulumiName("testName_", nil, nil, false))
	assert.Equal(t, "tESTNAME", TerraformToPulumiName("t_e_s_t_n_a_m_e", nil, nil, false))
	assert.Equal(t, "", TerraformToPulumiName("", nil, nil, true))
	assert.Equal(t, "Test", TerraformToPulumiName("test", nil, nil, true))
	assert.Equal(t, "TestName", TerraformToPulumiName("test_name", nil, nil, true))
	assert.Equal(t, "TestName_", TerraformToPulumiName("testName_", nil, nil, true))
	assert.Equal(t, "TESTNAME", TerraformToPulumiName("t_e_s_t_n_a_m_e", nil, nil, true))
}

func TestTerraformToPulumiNameWithSchemaInfoOverride(t *testing.T) {
	tfs := map[string]*schema.Schema{
		"list_property": {
			Type: schema.TypeList,
		},
	}

	// Override the List from the TF Schema with our own SchemaInfo overrides
	maxItemsOne := true
	ps := map[string]*SchemaInfo{
		"list_property": {
			MaxItemsOne: &maxItemsOne,
		},
	}

	name := TerraformToPulumiName("list_property", shimv1.NewSchema(tfs["list_property"]), ps["list_property"], false)
	if name != "listProperty" {
		t.Errorf("Expected `listProperty`, got %s", name)
	}
}

func TestPulumiToTerraformNameWithSchemaInfoOverride(t *testing.T) {
	tfs := map[string]*schema.Schema{
		"list_property": {
			Type: schema.TypeList,
		},
	}

	maxItemsOne := true
	ps := map[string]*SchemaInfo{
		"list_property": {
			MaxItemsOne: &maxItemsOne,
		},
	}

	name := PulumiToTerraformName("listProperty", shimv1.NewSchemaMap(tfs), ps)
	if name != "list_property" {
		t.Errorf("Expected `list_property`, got %s", name)
	}
}

func TestPluralize(t *testing.T) {
	tfs := map[string]*schema.Schema{
		"some_thing": {
			Type: schema.TypeSet,
		},
		"some_other_thing": {
			Type:     schema.TypeSet,
			MaxItems: 1,
		},
		"all_things": {
			Type: schema.TypeSet,
		},
	}

	terraformToPulumiName := func(k string) string {
		return TerraformToPulumiName(k, shimv1.NewSchema(tfs[k]), nil, false)
	}

	assert.Equal(t, "someThings", terraformToPulumiName("some_thing"))
	assert.Equal(t, "someOtherThing", terraformToPulumiName("some_other_thing"))
	assert.Equal(t, "allThings", terraformToPulumiName("all_things"))

	pulumiToTerraformName := func(k string) string {
		return PulumiToTerraformName(k, shimv1.NewSchemaMap(tfs), nil)
	}

	assert.Equal(t, "some_thing", pulumiToTerraformName("someThings"))
	assert.Equal(t, "some_other_things", pulumiToTerraformName("someOtherThings"))
	assert.Equal(t, "all_things", pulumiToTerraformName("allThings"))
}

func TestFromName(t *testing.T) {
	res1 := &PulumiResource{
		URN: "urn:pulumi:test::test::pkgA:index:t1::n1",
		Properties: resource.PropertyMap{
			"fifo": resource.NewBoolProperty(true),
		},
	}
	f1 := FromName(AutoNameOptions{
		Separator: "-",
		Maxlen:    80,
		Randlen:   7,
		PostTransform: func(res *PulumiResource, name string) (string, error) {
			if fifo, hasfifo := res.Properties["fifo"]; hasfifo {
				if fifo.IsBool() && fifo.BoolValue() {
					return name + ".fifo", nil
				}
			}
			return name, nil
		},
	})
	out1, err := f1(res1, 1)
	assert.NoError(t, err)
	assert.Len(t, out1, len("n1")+1+7+len(".fifo"))
	assert.True(t, strings.HasSuffix(out1.(string), ".fifo"))
}
