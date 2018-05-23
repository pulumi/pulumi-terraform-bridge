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
	"testing"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/stretchr/testify/assert"
)

func TestPulumiToTerraformName(t *testing.T) {
	assert.Equal(t, "", PulumiToTerraformName("", nil))
	assert.Equal(t, "test", PulumiToTerraformName("test", nil))
	assert.Equal(t, "test_name", PulumiToTerraformName("testName", nil))
	assert.Equal(t, "test_name_pascal", PulumiToTerraformName("TestNamePascal", nil))
	assert.Equal(t, "test_name", PulumiToTerraformName("test_name", nil))
	assert.Equal(t, "test_name_", PulumiToTerraformName("testName_", nil))
	assert.Equal(t, "t_e_s_t_n_a_m_e", PulumiToTerraformName("TESTNAME", nil))
}

func TestTerraformToPulumiName(t *testing.T) {
	assert.Equal(t, "", TerraformToPulumiName("", nil, false))
	assert.Equal(t, "test", TerraformToPulumiName("test", nil, false))
	assert.Equal(t, "testName", TerraformToPulumiName("test_name", nil, false))
	assert.Equal(t, "testName_", TerraformToPulumiName("testName_", nil, false))
	assert.Equal(t, "tESTNAME", TerraformToPulumiName("t_e_s_t_n_a_m_e", nil, false))
	assert.Equal(t, "", TerraformToPulumiName("", nil, true))
	assert.Equal(t, "Test", TerraformToPulumiName("test", nil, true))
	assert.Equal(t, "TestName", TerraformToPulumiName("test_name", nil, true))
	assert.Equal(t, "TestName_", TerraformToPulumiName("testName_", nil, true))
	assert.Equal(t, "TESTNAME", TerraformToPulumiName("t_e_s_t_n_a_m_e", nil, true))
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
	assert.Equal(t, "someThings", TerraformToPulumiName("some_thing", tfs["some_thing"], false))
	assert.Equal(t, "someOtherThing", TerraformToPulumiName("some_other_thing", tfs["some_other_thing"], false))
	assert.Equal(t, "allThings", TerraformToPulumiName("all_things", tfs["all_things"], false))

	assert.Equal(t, "some_thing", PulumiToTerraformName("someThings", tfs))
	assert.Equal(t, "some_other_things", PulumiToTerraformName("someOtherThings", tfs))
	assert.Equal(t, "all_things", PulumiToTerraformName("allThings", tfs))
}
