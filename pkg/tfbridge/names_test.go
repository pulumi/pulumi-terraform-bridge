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
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestPulumiToTerraformName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", PulumiToTerraformName("", nil, nil))
	assert.Equal(t, "test", PulumiToTerraformName("test", nil, nil))
	assert.Equal(t, "test_name", PulumiToTerraformName("testName", nil, nil))
	assert.Equal(t, "test_name_pascal", PulumiToTerraformName("TestNamePascal", nil, nil))
	assert.Equal(t, "test_name", PulumiToTerraformName("test_name", nil, nil))
	assert.Equal(t, "test_name_", PulumiToTerraformName("testName_", nil, nil))
	assert.Equal(t, "t_e_s_t_n_a_m_e", PulumiToTerraformName("TESTNAME", nil, nil))
}

func TestTerraformToPulumiName(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	tfs := map[string]*schema.Schema{
		"list_property": {
			Type: schema.TypeList,
		},
	}

	// Override the List from the TF Schema with our own SchemaInfo overrides
	ps := map[string]*SchemaInfo{
		"list_property": {
			MaxItemsOne: ref(true),
		},
	}

	name := TerraformToPulumiNameV2("list_property", shimv1.NewSchemaMap(tfs), ps)
	if name != "listProperty" {
		t.Errorf("Expected `listProperty`, got %s", name)
	}
}

func TestPulumiToTerraformNameWithSchemaInfoOverride(t *testing.T) {
	t.Parallel()
	tfs := map[string]*schema.Schema{
		"list_property": {
			Type: schema.TypeList,
		},
	}

	ps := map[string]*SchemaInfo{
		"list_property": {
			MaxItemsOne: ref(true),
		},
	}

	name := PulumiToTerraformName("listProperty", shimv1.NewSchemaMap(tfs), ps)
	if name != "list_property" {
		t.Errorf("Expected `list_property`, got %s", name)
	}
}

func TestPluralize(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	out1, err := f1(res1)
	assert.NoError(t, err)
	assert.Len(t, out1, len("n1")+1+7+len(".fifo"))
	assert.True(t, strings.HasSuffix(out1.(string), ".fifo"))
}

func TestFromNameSeedAndAutonaming(t *testing.T) {
	t.Parallel()
	res := &PulumiResource{
		URN:        "urn:pulumi:test::test::pkgA:index:t1::n1",
		Properties: resource.PropertyMap{},
		Seed:       []byte("test-seed"),
		Autonaming: &info.ComputeDefaultAutonamingOptions{
			ProposedName: "proposed-name",
			Mode:         info.ComputeDefaultAutonamingModePropose,
		},
	}

	f := FromName(AutoNameOptions{
		Separator: "-",
		Maxlen:    80,
		Randlen:   7,
	})

	out, err := f(res)
	assert.NoError(t, err)

	// Verify the output is a string and has expected format
	outStr := out.(string)
	assert.Equal(t, "proposed-name", outStr)
}

func TestBijectiveNameConversion(t *testing.T) {
	t.Parallel()

	certSchema := func() map[string]*schemav2.Schema {
		return map[string]*schemav2.Schema{
			"certificate_authority": {
				Type: schemav2.TypeList, Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{"data": {
						Type:     schema.TypeString,
						Computed: true,
					}},
				},
			},
			"certificate_authorities": {
				Type: schemav2.TypeList, Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{"data": {
						Type:     schema.TypeString,
						Computed: true,
					}},
				},
			},
		}
	}

	tests := []struct {
		schema   map[string]*schemav2.Schema
		info     map[string]*SchemaInfo
		expected map[string]string
	}{
		{ // Conflicting plural types
			schema: certSchema(),
			expected: map[string]string{
				"certificateAuthority":   "certificate_authority",
				"certificateAuthorities": "certificate_authorities",
			},
		},
		{ // Conflicting plural types with one made singular with MaxItemsOne
			schema: certSchema(),
			info: map[string]*SchemaInfo{
				"certificate_authority": {
					MaxItemsOne: BoolRef(true),
				},
			},
			expected: map[string]string{
				"certificateAuthority":   "certificate_authority",
				"certificateAuthorities": "certificate_authorities",
			},
		},
		{ // Respect .Name information
			schema: map[string]*schemav2.Schema{
				"singular_property": {
					Type: schemav2.TypeInt,
				},
				"plural_property": {
					Type: schemav2.TypeList,
					Elem: schemav2.TypeInt,
				},
			},
			info: map[string]*SchemaInfo{
				"singular_property": {
					Name: "singular",
				},
				"plural_property": {
					Name: "plural",
				},
			},
			expected: map[string]string{
				"singular": "singular_property",
				"plural":   "plural_property",
			},
		},
		{ // Check number mapping
			schema: map[string]*schemav2.Schema{
				"base_32_string_seed": {Type: schemav2.TypeString},
				"rfc_4180":            {Type: schemav2.TypeInt},
				"ext_100_with200":     {Type: schemav2.TypeInt},
			},
			expected: map[string]string{
				"base32StringSeed": "base_32_string_seed",
				"rfc4180":          "rfc_4180",
				"ext100With200":    "ext_100_with200",
			},
		},
		{ // Check manually overwritten remaps
			schema: map[string]*schemav2.Schema{
				"plural_type":  {Type: schemav2.TypeList},
				"plural_types": {Type: schemav2.TypeList},
			},
			info: map[string]*SchemaInfo{
				"plural_types": {Name: "remaped"},
			},
			expected: map[string]string{
				"pluralTypes": "plural_type",
				"remaped":     "plural_types",
			},
		},
		{ // Check when SchemaInfo is non-nil
			schema: map[string]*schemav2.Schema{
				"gallery_application":  {Type: schemav2.TypeList},
				"gallery_applications": {Type: schemav2.TypeList},
			},
			info: map[string]*SchemaInfo{
				"gallery_application":  {},
				"gallery_applications": {},
			},
			expected: map[string]string{
				"galleryApplication":  "gallery_application",
				"galleryApplications": "gallery_applications",
			},
		},
		{ // Check when SchemaInfo.MaxItemsOne: &false is set explicitly
			schema: map[string]*schemav2.Schema{
				"gallery_application":  {Type: schemav2.TypeList},
				"gallery_applications": {Type: schemav2.TypeList},
			},
			info: map[string]*SchemaInfo{
				"gallery_application":  {MaxItemsOne: ref(false)},
				"gallery_applications": {MaxItemsOne: ref(false)},
			},
			expected: map[string]string{
				"galleryApplication":  "gallery_application",
				"galleryApplications": "gallery_applications",
			},
		},
		{ // Check that users can override .MaxItems() == 1
			schema: map[string]*schemav2.Schema{
				"value": {Type: schemav2.TypeList, MaxItems: 1},
			},
			info: map[string]*SchemaInfo{
				"value": {MaxItemsOne: ref(false)},
			},
			expected: map[string]string{
				"values": "value",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		// Assert that forall (k, v) in tt.expected:
		// - PulumiToTerraformName(k) = v
		// - TerraformToPulumiName(v) = k
		t.Run("", func(t *testing.T) {
			require.Equal(t, len(tt.expected), len(tt.schema),
				"expected or schema misspecified")
			pulumiProps := make([]string, 0, len(tt.expected))
			tfAttributes := make([]string, 0, len(tt.expected))

			pulumiToTf := tt.expected
			tfToPulumi := map[string]string{}

			for k, v := range pulumiToTf {
				pulumiProps = append(pulumiProps, k)
				tfAttributes = append(tfAttributes, v)
				tfToPulumi[v] = k
			}
			sort.Slice(pulumiProps, func(i, j int) bool { return pulumiProps[i] < pulumiProps[j] })
			sort.Slice(tfAttributes, func(i, j int) bool { return tfAttributes[i] < tfAttributes[j] })
			assert.Equal(t, len(pulumiToTf), len(tfToPulumi), "map must be invertable")

			for _, tf := range tfAttributes {
				t.Run(tf+"->"+tfToPulumi[tf], func(t *testing.T) {
					m := shimv2.NewSchemaMap(tt.schema)
					assert.Equal(t, tfToPulumi[tf], TerraformToPulumiNameV2(tf, m, tt.info))
				})
			}
			for _, prop := range pulumiProps {
				t.Run(prop+"->"+pulumiToTf[prop], func(t *testing.T) {
					m := shimv2.NewSchemaMap(tt.schema)
					assert.Equal(t, pulumiToTf[prop], PulumiToTerraformName(prop, m, tt.info))
				})
			}
		})
	}
}
