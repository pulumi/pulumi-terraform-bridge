// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
