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

package tfbridge_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	ptokens "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestTokensSingleModule(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"foo_fizz_buzz":       nil,
				"foo_bar_hello_world": nil,
				"foo_bar":             nil,
			},
			DataSourcesMap: schema.ResourceMap{
				"foo_source1":             nil,
				"foo_very_special_source": nil,
			},
		}).Shim(),
		Resources: map[string]*tfbridge.ResourceInfo{
			"foo_bar": {Docs: &tfbridge.DocInfo{}},
		},
	}

	makeToken := func(module, name string) (string, error) {
		return fmt.Sprintf("foo:%s:%s", module, name), nil
	}
	opts := tokens.SingleModule("foo_", "index", makeToken)
	err := info.ComputeTokens(opts)
	require.NoError(t, err)

	expectedResources := map[string]*tfbridge.ResourceInfo{
		"foo_fizz_buzz":       {Tok: "foo:index:FizzBuzz"},
		"foo_bar_hello_world": {Tok: "foo:index:BarHelloWorld"},
		"foo_bar":             {Tok: "foo:index:Bar", Docs: &tfbridge.DocInfo{}},
	}
	expectedDatasources := map[string]*tfbridge.DataSourceInfo{
		"foo_source1":             {Tok: "foo:index:getSource1"},
		"foo_very_special_source": {Tok: "foo:index:getVerySpecialSource"},
	}

	assert.Equal(t, expectedResources, info.Resources)
	assert.Equal(t, expectedDatasources, info.DataSources)

	// Now test that overrides still work
	info.Resources = map[string]*tfbridge.ResourceInfo{
		"foo_bar_hello_world": {Tok: "foo:index:BarHelloPulumi"},
	}
	err = info.ComputeTokens(tfbridge.Strategy{
		Resource: opts.Resource,
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"foo_fizz_buzz":       {Tok: "foo:index:FizzBuzz"},
		"foo_bar_hello_world": {Tok: "foo:index:BarHelloPulumi"},
		"foo_bar":             {Tok: "foo:index:Bar"},
	}, info.Resources)
}

func TestTokensKnownModules(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"cs101_fizz_buzz_one_five": nil,
				"cs101_fizz_three":         nil,
				"cs101_fizz_three_six":     nil,
				"cs101_buzz_five":          nil,
				"cs101_buzz_ten":           nil,
				"cs101_game":               nil,
			},
		}).Shim(),
	}

	err := info.ComputeTokens(tfbridge.Strategy{
		Resource: tokens.KnownModules("cs101_", "index", []string{
			"fizz_", "buzz_", "fizz_buzz_",
		}, func(module, name string) (string, error) {
			return fmt.Sprintf("cs101:%s:%s", module, name), nil
		}).Resource,
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz:OneFive"},
		"cs101_fizz_three":         {Tok: "cs101:fizz:Three"},
		"cs101_fizz_three_six":     {Tok: "cs101:fizz:ThreeSix"},
		"cs101_buzz_five":          {Tok: "cs101:buzz:Five"},
		"cs101_buzz_ten":           {Tok: "cs101:buzz:Ten"},
		"cs101_game":               {Tok: "cs101:index:Game"},
	}, info.Resources)
}

func TestTokensKnownModulesAlreadyMapped(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"pkg_m1_fizz": nil,
				"pkg_m2_buzz": nil,
				"not_pkg_res": nil,
			},
		}).Shim(),
		Resources: map[string]*tfbridge.ResourceInfo{
			"pkg_m2_buzz": {Tok: "pkg:index:Buzz"},
			"not_pkg_res": {Tok: "not:pkg:Res"},
		},
	}

	err := info.ComputeTokens(tokens.KnownModules(
		"pkg_", "", []string{"m1"}, tokens.MakeStandard("pkg")))
	require.NoError(t, err)
	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"pkg_m1_fizz": {Tok: "pkg:m1/fizz:Fizz"},
		"pkg_m2_buzz": {Tok: "pkg:index:Buzz"},
		"not_pkg_res": {Tok: "not:pkg:Res"},
	}, info.Resources)
}

func TestTokensMappedModules(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"cs101_fizz_buzz_one_five": nil,
				"cs101_fizz_three":         nil,
				"cs101_fizz_three_six":     nil,
				"cs101_buzz_five":          nil,
				"cs101_buzz_ten":           nil,
				"cs101_game":               nil,
			},
		}).Shim(),
	}
	err := info.ComputeTokens(tfbridge.Strategy{
		Resource: tokens.MappedModules("cs101_", "idx", map[string]string{
			"fizz_":      "fIzZ",
			"buzz_":      "buZZ",
			"fizz_buzz_": "fizZBuzz",
		}, func(module, name string) (string, error) {
			return fmt.Sprintf("cs101:%s:%s", module, name), nil
		}).Resource,
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizZBuzz:OneFive"},
		"cs101_fizz_three":         {Tok: "cs101:fIzZ:Three"},
		"cs101_fizz_three_six":     {Tok: "cs101:fIzZ:ThreeSix"},
		"cs101_buzz_five":          {Tok: "cs101:buZZ:Five"},
		"cs101_buzz_ten":           {Tok: "cs101:buZZ:Ten"},
		"cs101_game":               {Tok: "cs101:idx:Game"},
	}, info.Resources)
}

func TestUnmappable(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"cs101_fizz_buzz_one_five": nil,
				"cs101_fizz_three":         nil,
				"cs101_fizz_three_six":     nil,
				"cs101_buzz_five":          nil,
				"cs101_buzz_ten":           nil,
				"cs101_game":               nil,
			},
		}).Shim(),
	}

	strategy := tokens.KnownModules("cs101_", "index", []string{
		"fizz_", "buzz_", "fizz_buzz_",
	}, func(module, name string) (string, error) {
		return fmt.Sprintf("cs101:%s:%s", module, name), nil
	})
	strategy = strategy.Ignore("five")
	err := info.ComputeTokens(strategy)
	assert.NoError(t, err)
	assert.Nilf(t, info.Resources["cs101_buzz_five"],
		`We told the strategy to ignore tokens containing "five"`)

	// Override the unmappable resources
	info.Resources = map[string]*tfbridge.ResourceInfo{
		// When "five" comes after another number, we print it as "5"
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz:One5"},
		"cs101_buzz_five":          {Tok: "cs101:buzz:Five"},
	}
	err = info.ComputeTokens(strategy)
	assert.NoError(t, err)
	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz:One5"},
		"cs101_fizz_three":         {Tok: "cs101:fizz:Three"},
		"cs101_fizz_three_six":     {Tok: "cs101:fizz:ThreeSix"},
		"cs101_buzz_five":          {Tok: "cs101:buzz:Five"},
		"cs101_buzz_ten":           {Tok: "cs101:buzz:Ten"},
		"cs101_game":               {Tok: "cs101:index:Game"},
	}, info.Resources)
}

func TestMapperExistingToken(t *testing.T) {
	provider := func() *tfbridge.ProviderInfo {
		return &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				ResourcesMap: schema.ResourceMap{
					"pkg_mod1_r1": nil,
					"pkg_mod1_r2": nil,
					"pkg_mod1_r3": nil,
					"pkg_mod2_r1": nil,
				},
				DataSourcesMap: schema.ResourceMap{
					"pkg_mod1_d1": nil,
				},
			}).Shim(),
			Resources: map[string]*tfbridge.ResourceInfo{
				"pkg_mod2_r1": {Tok: "AlreadyMapped."},
				"pkg_mod1_r3": {Tok: "AlreadyMapped.."},
			},
			IgnoreMappings: []string{"pkg_mod1_r2"},
		}
	}

	type tk struct{ module, name string }
	finalize := func(t *testing.T, expectedTokens ...tk) tokens.Make {
		return func(module, name string) (string, error) {
			assert.Containsf(t, expectedTokens, tk{module, name},
				"Finalize called on unexpected token")
			return tokens.MakeStandard("pkg")(module, name)
		}
	}

	// Ensure that we don't call finalize for tokens where we won't use the finalized
	// value. This allows provider authors to write code that will error or panic on
	// tokens that they can be reasonably certain it won't be called on.

	t.Run("singe", func(t *testing.T) {
		provider().MustComputeTokens(tokens.SingleModule("pkg_", "index",
			finalize(t, tk{"index", "Mod1R1"}, tk{"index", "getMod1D1"})))
	})
	t.Run("known", func(t *testing.T) {
		provider().MustComputeTokens(tokens.KnownModules("pkg_", "index",
			[]string{"mod1"},
			finalize(t, tk{"mod1", "R1"}, tk{"mod1", "getD1"})))
	})
	t.Run("mapped", func(t *testing.T) {
		provider().MustComputeTokens(tokens.MappedModules("pkg_", "index",
			map[string]string{"mod1": "SomeMod"},
			finalize(t, tk{"SomeMod", "R1"}, tk{"SomeMod", "getD1"})))
	})
}

func TestIgnored(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			ResourcesMap: schema.ResourceMap{
				"cs101_one_five":  nil,
				"cs101_three":     nil,
				"cs101_three_six": nil,
			},
		}).Shim(),
		IgnoreMappings: []string{"cs101_three"},
	}
	err := info.ComputeTokens(tokens.SingleModule("cs101_", "index_", tokens.MakeStandard("cs101")))
	assert.NoError(t, err)
	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_one_five":  {Tok: "cs101:index/oneFive:OneFive"},
		"cs101_three_six": {Tok: "cs101:index/threeSix:ThreeSix"},
	}, info.Resources)
}

func TestTokensInferredModules(t *testing.T) {
	tests := []struct {
		name            string
		resourceMapping map[string]string
		opts            *tokens.InferredModulesOpts
	}{
		{
			name: "oci-example",
			// Motivating example and explanation:
			//
			// The algorithm only has the list of token names to work off
			// of. It doesn't know what modules should exist, so it needs to
			// figure out.
			//
			// Tokens can be cleanly divided into segments at '_'
			// boundaries. However, its unclear how many segments make up the
			// module, and how many segments make up the name.
			//
			// Giving a concrete example, the algorithm needs to figure out
			// what Pulumi token to give the Terraform token
			// oci_apm_apm_domain:
			//
			//   Dividing into segments, the algorithm has module [apm apm
			//   domain] and name [], written [apm apm domain]:[].
			//
			//   It starts by considering all token segments as part of the
			//   module name. Examining the module [apm apm domain], the
			//   algorithm notices that there are not enough objects in the
			//   [apm apm domain] module to satisfy MinimumModuleSize. It then
			//   downshifts the perspective token to [apm apm]:[domain].
			//
			//   The algorithm will process all tokens with modules that start
			//   with [apm apm $NEXT] for all $NEXT before it reconsiders the
			//   [apm apm] module.
			//
			//   Next iteration, the algorithm considers [apm
			//   apm]:[domain]. Because the [apm apm] module has only 1
			//   member, the algorithm downshifs [apm apm]:[domain] to
			//   [apm]:[apm domain].
			//
			//   Next iteration, the algorithm sees 2 different tokens within
			//   the [apm] module: [apm]:[apm domain] and [apm]:[sub
			//   domain]. Since 2 >= MinimumModuleSize, the algorithm
			//   finalizes both tokens into apm:ApmDomain and apm:SubDomain
			//   respectively.
			//
			//
			// The process is unstable for insertion: if the user added
			// "oci_apm_apm_thingy" resource, then there'd be two entries and
			// it might decide oci_apm_apm is now a module.
			resourceMapping: map[string]string{
				"oci_adm_knowledge_base": "index:AdmKnowledgeBase",
				"oci_apm_apm_domain":     "apm:ApmDomain",
				"oci_apm_sub_domain":     "apm:SubDomain",

				"oci_apm_config_config": "apmConfig:Config",
				"oci_apm_config_user":   "apmConfig:User",

				"oci_apm_synthetics_monitor":                 "apmSynthetics:Monitor",
				"oci_apm_synthetics_script":                  "apmSynthetics:Script",
				"oci_apm_synthetics_dedicated_vantage_point": "apmSynthetics:DedicatedVantagePoint",
			},
			opts: &tokens.InferredModulesOpts{
				TfPkgPrefix:          "oci_",
				MinimumModuleSize:    2,
				MimimumSubmoduleSize: 2,
			},
		},
		{
			name: "non-overlapping mapping",
			resourceMapping: map[string]string{
				"pkg_foo_bar":             "index:FooBar",
				"pkg_fizz_buzz":           "index:FizzBuzz",
				"pkg_resource":            "index:Resource",
				"pkg_very_long_name":      "index:VeryLongName",
				"pkg_very_very_long_name": "index:VeryVeryLongName",
			},
		},
		{
			name: "detect a simple module",
			resourceMapping: map[string]string{
				"pkg_hello_world":   "hello:World",
				"pkg_hello_pulumi":  "hello:Pulumi",
				"pkg_hello":         "hello:Hello",
				"pkg_goodbye_folks": "index:GoodbyeFolks",
				"pkg_hi":            "index:Hi",
			},
			opts: &tokens.InferredModulesOpts{
				// We set MinimumModuleSize down to 3 to so we only need
				// tree entries prefixed with `pkg_hello` to have a hello
				// module created.
				MinimumModuleSize: 3,
			},
		},
		{
			name: "nested modules",
			resourceMapping: map[string]string{
				"pkg_mod_r1":     "mod:R1",
				"pkg_mod_r2":     "mod:R2",
				"pkg_mod_r3":     "mod:R3",
				"pkg_mod_r4":     "mod:R4",
				"pkg_mod_sub_r1": "modSub:R1",
				"pkg_mod_sub_r2": "modSub:R2",
				"pkg_mod_sub_r3": "modSub:R3",
				"pkg_mod_sub_r4": "modSub:R4",
				"pkg_mod_not_r1": "mod:NotR1",
				"pkg_mod_not_r2": "mod:NotR2",
			},
			opts: &tokens.InferredModulesOpts{
				TfPkgPrefix: "pkg_",
				// We set the minimum module size to 4. This ensures that
				// `pkg_mod` is picked up as a module.
				MinimumModuleSize: 4,
				// We set the MimimumSubmoduleSize to 3, ensuring that
				// `pkg_mod_sub_*` is is given its own `modSub` module (4
				// elements), while `pkg_mod_not_*` is put in the `mod`
				// module, since `pkg_mod_not` only has 2 elements.
				MimimumSubmoduleSize: 3,
			},
		},
		{
			name: "nested-collapse",
			resourceMapping: map[string]string{
				"pkg_mod_r1":     "mod:R1",
				"pkg_mod_r2":     "mod:R2",
				"pkg_mod_sub_r1": "mod:SubR1",
				"pkg_mod_sub_r2": "mod:SubR2",
			},
			opts: &tokens.InferredModulesOpts{
				TfPkgPrefix:          "pkg_",
				MinimumModuleSize:    4,
				MimimumSubmoduleSize: 3,
			},
		},
		{
			name: "module and item",
			resourceMapping: map[string]string{
				"pkg_mod":    "mod:Mod",
				"pkg_mod_r1": "mod:R1",
				"pkg_mod_r2": "mod:R2",
				"pkg_r1":     "index:R1",
			},
			opts: &tokens.InferredModulesOpts{
				MinimumModuleSize: 3,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			resources := schema.ResourceMap{}
			for k := range tt.resourceMapping {
				resources[k] = nil
			}
			info := &tfbridge.ProviderInfo{
				P: (&schema.Provider{
					ResourcesMap: resources,
				}).Shim(),
			}

			strategy, err := tokens.InferredModules(info,
				func(module, name string) (string, error) { return module + ":" + name, nil },
				tt.opts)
			require.NoError(t, err)
			err = info.ComputeTokens(strategy)
			require.NoError(t, err)

			mapping := map[string]string{}
			for k, v := range info.Resources {
				mapping[k] = v.Tok.String()
			}
			assert.Equal(t, tt.resourceMapping, mapping)
		})
	}
}

func makeAutoAliasing(t *testing.T) (
	*md.Data, func(*tfbridge.ProviderInfo, tfbridge.ProviderMetadata),
) {
	metadata, err := metadata.New(nil)
	require.NoError(t, err)

	return metadata, func(prov *tfbridge.ProviderInfo, metadata tfbridge.ProviderMetadata) {
		prov.MetadataInfo = &tfbridge.MetadataInfo{Data: metadata, Path: "must be non-empty"}
		err := prov.ApplyAutoAliases()
		require.NoError(t, err)
	}
}

func TestTokenAliasing(t *testing.T) {
	// We run this test multiple times to guard against nondeterminism.
	//
	// See https://github.com/pulumi/pulumi-terraform-bridge/issues/1286.
	for i := 0; i < 500; i++ {
		t.Run(fmt.Sprintf("%d", i), testTokenAliasing)
	}
}

func TestTokenAliasing1(t *testing.T) {
	testTokenAliasing(t)
}

func testTokenAliasing(t *testing.T) {
	t.Parallel()
	provider := func() *tfbridge.ProviderInfo {
		return &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				ResourcesMap: schema.ResourceMap{
					"pkg_mod1_r1": nil,
					"pkg_mod1_r2": nil,
					"pkg_mod2_r1": nil,
				},
				DataSourcesMap: schema.ResourceMap{
					"pkg_mod1_d1": nil,
				},
			}).Shim(),
		}
	}

	simple := provider()
	simple.Version = "1.0.0"

	metadata, autoAliasing := makeAutoAliasing(t)

	err := simple.ComputeTokens(tokens.SingleModule("pkg_", "index", tokens.MakeStandard("pkg")))
	require.NoError(t, err)

	autoAliasing(simple, metadata)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"pkg_mod1_r1": {Tok: "pkg:index/mod1R1:Mod1R1"},
		"pkg_mod1_r2": {Tok: "pkg:index/mod1R2:Mod1R2"},
		"pkg_mod2_r1": {Tok: "pkg:index/mod2R1:Mod2R1"},
	}, simple.Resources)

	modules := provider()
	modules.Version = "1.0.0"

	knownModules := tokens.KnownModules("pkg_", "",
		[]string{"mod1", "mod2"}, tokens.MakeStandard("pkg"))

	err = modules.ComputeTokens(knownModules)
	require.NoError(t, err)

	autoAliasing(modules, metadata)
	require.NoError(t, err)

	hist2 := md.Clone(metadata)
	ref := func(s string) *string { return &s }
	require.Equal(t, map[string]*tfbridge.ResourceInfo{
		"pkg_mod1_r1": {
			Tok:     "pkg:mod1/r1:R1",
			Aliases: []tfbridge.AliasInfo{{Type: ref("pkg:index/mod1R1:Mod1R1")}},
		},
		"pkg_mod1_r1_legacy": {
			Tok:                "pkg:index/mod1R1:Mod1R1",
			DeprecationMessage: "pkg.index/mod1r1.Mod1R1 has been deprecated in favor of pkg.mod1/r1.R1",
			Docs:               &tfbridge.DocInfo{Source: "kg_mod1_r1.html.markdown"},
		},
		"pkg_mod1_r2": {
			Tok:     "pkg:mod1/r2:R2",
			Aliases: []tfbridge.AliasInfo{{Type: ref("pkg:index/mod1R2:Mod1R2")}},
		},
		"pkg_mod1_r2_legacy": {
			Tok:                "pkg:index/mod1R2:Mod1R2",
			DeprecationMessage: "pkg.index/mod1r2.Mod1R2 has been deprecated in favor of pkg.mod1/r2.R2",
			Docs:               &tfbridge.DocInfo{Source: "kg_mod1_r2.html.markdown"},
		},
		"pkg_mod2_r1": {
			Tok:     "pkg:mod2/r1:R1",
			Aliases: []tfbridge.AliasInfo{{Type: ref("pkg:index/mod2R1:Mod2R1")}},
		},
		"pkg_mod2_r1_legacy": {
			Tok:                "pkg:index/mod2R1:Mod2R1",
			DeprecationMessage: "pkg.index/mod2r1.Mod2R1 has been deprecated in favor of pkg.mod2/r1.R1",
			Docs:               &tfbridge.DocInfo{Source: "kg_mod2_r1.html.markdown"},
		},
	}, modules.Resources)
	assert.Equal(t, map[string]*tfbridge.DataSourceInfo{
		"pkg_mod1_d1": {Tok: "pkg:mod1/getD1:getD1"},
		"pkg_mod1_d1_legacy": {
			Tok:                "pkg:index/getMod1D1:getMod1D1",
			DeprecationMessage: "pkg.index/getmod1d1.getMod1D1 has been deprecated in favor of pkg.mod1/getd1.getD1",
			Docs:               &tfbridge.DocInfo{Source: "kg_mod1_d1.html.markdown"},
		},
	},
		modules.DataSources)

	modules2 := provider()
	modules2.Version = "1.0.0"

	err = modules2.ComputeTokens(knownModules)
	require.NoError(t, err)

	autoAliasing(modules2, metadata)
	require.NoError(t, err)

	hist3 := md.Clone(metadata)
	assert.Equal(t, string(hist2.Marshal()), string(hist3.Marshal()),
		"No changes should imply no change in history")
	assert.Equal(t, modules, modules2)
	assert.Equalf(t, func() string {
		m := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(hist2.Marshal(), &m))
		resources := m["auto-aliasing"].(map[string]interface{})["resources"]
		pkgMod2R1 := resources.(map[string]interface{})["pkg_mod2_r1"]
		return pkgMod2R1.(map[string]interface{})["current"].(string)
	}(), "pkg:mod2/r1:R1", "Ensure current holds the most recent name")

	modules3 := provider()
	modules3.Version = "100.0.0"

	err = modules3.ComputeTokens(knownModules)
	require.NoError(t, err)

	autoAliasing(modules3, metadata)
	require.NoError(t, err)

	assert.Equalf(t, map[string]*tfbridge.ResourceInfo{
		"pkg_mod1_r1": {
			Tok:     "pkg:mod1/r1:R1",
			Aliases: []tfbridge.AliasInfo{{Type: ref("pkg:index/mod1R1:Mod1R1")}},
		},
		"pkg_mod1_r2": {
			Tok:     "pkg:mod1/r2:R2",
			Aliases: []tfbridge.AliasInfo{{Type: ref("pkg:index/mod1R2:Mod1R2")}},
		},
		"pkg_mod2_r1": {
			Tok:     "pkg:mod2/r1:R1",
			Aliases: []tfbridge.AliasInfo{{Type: ref("pkg:index/mod2R1:Mod2R1")}},
		},
	}, modules3.Resources, "All hard aliases should be removed on a major version upgrade")

	// A provider with no version should assume the most recent major
	// version in history – in this case, all aliases should be kept
	modules4 := provider()

	err = modules4.ComputeTokens(knownModules)
	require.NoError(t, err)

	autoAliasing(modules4, metadata)
	require.NoError(t, err)
	assert.Equal(t, modules.Resources, modules4.Resources)
}

func TestMaxItemsOneAliasing(t *testing.T) {
	provider := func(f1, f2 bool) *tfbridge.ProviderInfo {
		prov := &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				ResourcesMap: schema.ResourceMap{
					"pkg_r1": (&schema.Resource{Schema: schema.SchemaMap{
						"f1": Schema{MaxItemsOne: f1},
						"f2": Schema{MaxItemsOne: f2},
						"f3": Schema{typ: shim.TypeString},
					}}).Shim(),
				},
			}).Shim(),
		}
		err := prov.ComputeTokens(tokens.SingleModule("pkg_", "index", tokens.MakeStandard("pkg")))
		require.NoError(t, err)
		return prov
	}
	info := provider(true, false)
	metadata, autoAliasing := makeAutoAliasing(t)

	// Save current state into metadata
	autoAliasing(info, metadata)

	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemsOne": true
                    },
                    "f2": {
                        "maxItemsOne": false
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string(metadata.MarshalIndent()))

	info = provider(false, true)

	// Apply metadata back into the provider
	autoAliasing(info, metadata)

	assert.True(t, *info.Resources["pkg_r1"].Fields["f1"].MaxItemsOne)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f2"].MaxItemsOne)

	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemsOne": true
                    },
                    "f2": {
                        "maxItemsOne": false
                    }
                }
            }
        }
    },
    "auto-settings": {
        "resources": {
            "pkg_r1": {
                "maxItemsOneOverrides": {
                    "f1": true,
                    "f2": false
                }
            }
        }
    }
}`).Equal(t, string(metadata.MarshalIndent()))

	// Apply metadata back into the provider again, making sure there isn't a diff
	autoAliasing(info, metadata)

	assert.True(t, *info.Resources["pkg_r1"].Fields["f1"].MaxItemsOne)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f2"].MaxItemsOne)

	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemsOne": true
                    },
                    "f2": {
                        "maxItemsOne": false
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string(metadata.MarshalIndent()))

	// Validate that overrides work

	info = provider(true, false)
	info.Resources["pkg_r1"].Fields = map[string]*tfbridge.SchemaInfo{
		"f1": {MaxItemsOne: tfbridge.False()},
	}

	autoAliasing(info, metadata)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f1"].MaxItemsOne)

	f2Schema := info.P.ResourcesMap().Get("pkg_r1").Schema().Get("f2")
	f2Overrides := info.Resources["pkg_r1"].Fields["f2"]

	assert.False(t, tfbridge.IsMaxItemsOne(f2Schema, f2Overrides))

	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemsOne": false
                    },
                    "f2": {
                        "maxItemsOne": false
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string(metadata.MarshalIndent()))
}

func TestMaxItemsOneAliasingExpiring(t *testing.T) {
	provider := func(f1, f2 bool) *tfbridge.ProviderInfo {
		prov := &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				ResourcesMap: schema.ResourceMap{
					"pkg_r1": (&schema.Resource{Schema: schema.SchemaMap{
						"f1": Schema{MaxItemsOne: f1},
						"f2": Schema{MaxItemsOne: f2},
					}}).Shim(),
				},
			}).Shim(),
		}
		err := prov.ComputeTokens(tokens.SingleModule("pkg_", "index", tokens.MakeStandard("pkg")))
		require.NoError(t, err)
		return prov
	}
	info := provider(true, false)
	metadata, autoAliasing := makeAutoAliasing(t)

	// Save current state into metadata
	autoAliasing(info, metadata)

	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemsOne": true
                    },
                    "f2": {
                        "maxItemsOne": false
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string(metadata.MarshalIndent()))

	info = provider(false, true)

	// Apply metadata back into the provider
	info.Version = "1.0.0" // New major version
	autoAliasing(info, metadata)

	assert.Nil(t, info.Resources["pkg_r1"].Fields["f1"])
	assert.Nil(t, info.Resources["pkg_r1"].Fields["f2"])
	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "majorVersion": 1,
                "fields": {
                    "f1": {
                        "maxItemsOne": false
                    },
                    "f2": {
                        "maxItemsOne": true
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string(metadata.MarshalIndent()))
}

func TestMaxItemsOneAliasingNested(t *testing.T) {
	provider := func(f1, f2 bool) *tfbridge.ProviderInfo {
		prov := &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				ResourcesMap: schema.ResourceMap{
					"pkg_r1": (&schema.Resource{Schema: schema.SchemaMap{
						"f1": Schema{},
						"f2": Schema{elem: (&schema.Resource{
							Schema: schema.SchemaMap{
								"n1": Schema{MaxItemsOne: f1},
								"n2": Schema{MaxItemsOne: f2},
							},
						}).Shim()},
					}}).Shim(),
				},
			}).Shim(),
		}
		err := prov.ComputeTokens(tokens.SingleModule("pkg_", "index", tokens.MakeStandard("pkg")))
		require.NoError(t, err)
		return prov
	}
	info := provider(true, false)
	metadata, autoAliasing := makeAutoAliasing(t)

	// Save current state into metadata
	autoAliasing(info, metadata)

	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemsOne": false
                    },
                    "f2": {
                        "maxItemsOne": false,
                        "elem": {
                            "fields": {
                                "n1": {
                                    "maxItemsOne": true
                                },
                                "n2": {
                                    "maxItemsOne": false
                                }
                            }
                        }
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string(metadata.MarshalIndent()))

	// Apply the saved metadata to a new provider
	info = provider(false, true)
	autoAliasing(info, metadata)

	autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemsOne": false
                    },
                    "f2": {
                        "maxItemsOne": false,
                        "elem": {
                            "fields": {
                                "n1": {
                                    "maxItemsOne": true
                                },
                                "n2": {
                                    "maxItemsOne": false
                                }
                            }
                        }
                    }
                }
            }
        }
    },
    "auto-settings": {
        "resources": {
            "pkg_r1": {
                "maxItemsOneOverrides": {
                    "f2.$.n1": true,
                    "f2.$.n2": false
                }
            }
        }
    }
}`).Equal(t, string(metadata.MarshalIndent()))

	assert.True(t, *info.Resources["pkg_r1"].Fields["f2"].Elem.Fields["n1"].MaxItemsOne)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f2"].Elem.Fields["n2"].MaxItemsOne)
}

// (*ProviderInfo).SetAutonaming skips fields that have a SchemaInfo already defined in
// their resource's ResourceInfo.Fields. We need to make sure that unless we mark a field
// as `MaxItemsOne: nonNil` for some non-nil value, we don't leave that field entry behind
// since that will disable SetAutonaming.
func TestMaxItemsOneAliasingWithAutoNaming(t *testing.T) {
	provider := func() *tfbridge.ProviderInfo {
		info, err := metadata.New(nil)
		require.NoError(t, err)

		prov := &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				ResourcesMap: schema.ResourceMap{
					"pkg_r1": (&schema.Resource{Schema: schema.SchemaMap{
						"name":      Schema{typ: shim.TypeString},
						"nest_list": Schema{elem: Schema{typ: shim.TypeBool}},
						"nest_flat": Schema{
							elem:        Schema{typ: shim.TypeBool},
							MaxItemsOne: true,
						},
						"override_list": Schema{elem: Schema{typ: shim.TypeBool}},
						"override_flat": Schema{
							elem:        Schema{typ: shim.TypeInt},
							MaxItemsOne: true,
						},
					}}).Shim(),
				},
			}).Shim(),
			MetadataInfo: &tfbridge.MetadataInfo{Data: info, Path: "must be non-empty"},
		}
		err = prov.ComputeTokens(tokens.SingleModule("pkg_", "index", tokens.MakeStandard("pkg")))
		require.NoError(t, err)
		return prov
	}

	assertExpected := func(t *testing.T, p *tfbridge.ProviderInfo) {
		r := p.Resources["pkg_r1"]
		assert.True(t, r.Fields["name"].Default.AutoNamed)

		assert.Nil(t, r.Fields["nest_list"])
		assert.Nil(t, r.Fields["override_list"])

		autogold.Expect(`{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "nest_flat": {
                        "maxItemsOne": true
                    },
                    "nest_list": {
                        "maxItemsOne": false
                    },
                    "override_flat": {
                        "maxItemsOne": true
                    },
                    "override_list": {
                        "maxItemsOne": false
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string((*md.Data)(p.MetadataInfo.Data).MarshalIndent()))
	}

	t.Run("auto-named-then-aliased", func(t *testing.T) {
		p := provider()

		p.SetAutonaming(24, "-")
		err := p.ApplyAutoAliases()
		require.NoError(t, err)

		assertExpected(t, p)
	})

	t.Run("auto-aliased-then-named", func(t *testing.T) {
		p := provider()
		err := p.ApplyAutoAliases()
		require.NoError(t, err)
		p.SetAutonaming(24, "-")

		assertExpected(t, p)
	})
}

func TestMaxItemsOneDataSourceAliasing(t *testing.T) {
	provider := func() *tfbridge.ProviderInfo {
		info, err := metadata.New(nil)
		require.NoError(t, err)

		prov := &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				DataSourcesMap: schema.ResourceMap{
					"pkg_r1": (&schema.Resource{Schema: schema.SchemaMap{
						"name":      Schema{typ: shim.TypeString},
						"nest_list": Schema{elem: Schema{typ: shim.TypeBool}},
						"nest_flat": Schema{
							elem:        Schema{typ: shim.TypeBool},
							MaxItemsOne: true,
						},
						"override_list": Schema{elem: Schema{typ: shim.TypeBool}},
						"override_flat": Schema{
							elem:        Schema{typ: shim.TypeInt},
							MaxItemsOne: true,
						},
					}}).Shim(),
				},
			}).Shim(),
			MetadataInfo: &tfbridge.MetadataInfo{Data: info, Path: "must be non-empty"},
		}
		err = prov.ComputeTokens(tokens.SingleModule("pkg_", "index", tokens.MakeStandard("pkg")))
		require.NoError(t, err)
		return prov
	}

	assertExpected := func(t *testing.T, p *tfbridge.ProviderInfo) {
		r := p.DataSources["pkg_r1"]

		assert.Nil(t, r.Fields["nest_list"])
		assert.Nil(t, r.Fields["override_list"])

		autogold.Expect(`{
    "auto-aliasing": {
        "datasources": {
            "pkg_r1": {
                "current": "pkg:index/getR1:getR1",
                "fields": {
                    "nest_flat": {
                        "maxItemsOne": true
                    },
                    "nest_list": {
                        "maxItemsOne": false
                    },
                    "override_flat": {
                        "maxItemsOne": true
                    },
                    "override_list": {
                        "maxItemsOne": false
                    }
                }
            }
        }
    },
    "auto-settings": {}
}`).Equal(t, string((*md.Data)(p.MetadataInfo.Data).MarshalIndent()))
	}

	t.Run("auto-named-then-aliased", func(t *testing.T) {
		p := provider()

		p.SetAutonaming(24, "-")
		err := p.ApplyAutoAliases()
		require.NoError(t, err)

		assertExpected(t, p)
	})

	t.Run("auto-aliased-then-named", func(t *testing.T) {
		p := provider()
		err := p.ApplyAutoAliases()
		require.NoError(t, err)
		p.SetAutonaming(24, "-")

		assertExpected(t, p)
	})
}

func TestAutoAliasingChangeDataSources(t *testing.T) {
	provider := func(t *testing.T, meta string, n int) *tfbridge.ProviderInfo {
		dsName := ptokens.ModuleMember(fmt.Sprintf("pkg:index:getD%d", n))
		info, err := metadata.New([]byte(meta))
		require.NoError(t, err)

		prov := &tfbridge.ProviderInfo{
			Version: "1.0.0",
			P: (&schema.Provider{
				DataSourcesMap: schema.ResourceMap{
					"pkg_d1": (&schema.Resource{}).Shim(),
				},
			}).Shim(),
			DataSources: map[string]*tfbridge.DataSourceInfo{
				"pkg_d1": {Tok: dsName},
			},
			MetadataInfo: &tfbridge.MetadataInfo{Data: info, Path: "must be non-empty"},
		}
		err = prov.ApplyAutoAliases()
		require.NoError(t, err)
		return prov
	}

	meta1 := `{
        "auto-aliasing": {
            "datasources": {
                "pkg_d1": {
                    "current": "pkg:index:getD1",
                    "majorVersion": 1
                }
            }
        }
    }`

	meta2 := `{
        "auto-aliasing": {
            "datasources": {
                "pkg_d1": {
                    "current": "pkg:index:getD2",
                    "majorVersion": 1,
                    "past": [
                        {
                            "name": "pkg:index:getD1",
                            "inCodegen": false,
                            "majorVersion": 1
                        }
                    ]
                }
            }
        }
    }`

	testCases := []struct {
		testname string
		name     int
		current  string
		expect   autogold.Value
	}{
		// Test that ApplyAutoAliases will update current and append history.
		{"confirm-change", 2, meta1, autogold.Expect(`{
    "auto-aliasing": {
        "datasources": {
            "pkg_d1": {
                "current": "pkg:index:getD2",
                "past": [
                    {
                        "name": "pkg:index:getD1",
                        "inCodegen": true,
                        "majorVersion": 1
                    }
                ],
                "majorVersion": 1
            }
        }
    },
    "auto-settings": {
        "datasources": {
            "pkg_d1": {
                "renames": [
                    "pkg:index:getD1"
                ]
            }
        }
    }
}`)},

		// Test that we don't keep redundant history.
		{"reversion", 1, meta2, autogold.Expect(`{
    "auto-aliasing": {
        "datasources": {
            "pkg_d1": {
                "current": "pkg:index:getD1",
                "past": [
                    {
                        "name": "pkg:index:getD2",
                        "inCodegen": true,
                        "majorVersion": 1
                    }
                ],
                "majorVersion": 1
            }
        }
    },
    "auto-settings": {
        "datasources": {
            "pkg_d1": {
                "renames": [
                    "pkg:index:getD2"
                ]
            }
        }
    }
}`)},

		// Test that adding a name that has already been seen works as expected.
		{"add-past", 3, meta2, autogold.Expect(`{
    "auto-aliasing": {
        "datasources": {
            "pkg_d1": {
                "current": "pkg:index:getD3",
                "past": [
                    {
                        "name": "pkg:index:getD1",
                        "inCodegen": false,
                        "majorVersion": 1
                    },
                    {
                        "name": "pkg:index:getD2",
                        "inCodegen": true,
                        "majorVersion": 1
                    }
                ],
                "majorVersion": 1
            }
        }
    },
    "auto-settings": {
        "datasources": {
            "pkg_d1": {
                "renames": [
                    "pkg:index:getD1",
                    "pkg:index:getD2"
                ]
            }
        }
    }
}`)},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testname, func(t *testing.T) {
			p := provider(t, tc.current, tc.name)
			st := string((*md.Data)(p.MetadataInfo.Data).MarshalIndent())
			tc.expect.Equal(t, st)

			// Regardless of the input and output, once we apply some name to
			// our state, reapplying the same name to the new state should be
			// idempotent.
			t.Run("idempotent", func(t *testing.T) {
				p := provider(t, st, tc.name)
				assert.Equal(t, st, string((*md.Data)(p.MetadataInfo.Data).MarshalIndent()))
			})
		})
	}
}

func TestDeletedResourcesAutoAliasing(t *testing.T) {
	provider := func(t *testing.T, tok string, resourceMap schema.ResourceMap, meta []byte) *tfbridge.ProviderInfo {
		info, err := metadata.New(meta)
		require.NoError(t, err)

		prov := &tfbridge.ProviderInfo{
			Version: "1.0.0",
			P: (&schema.Provider{
				ResourcesMap: resourceMap,
			}).Shim(),
			Resources: map[string]*tfbridge.ResourceInfo{
				"pkg_r1": {
					Tok:  ptokens.Type(tok),
					Docs: &tfbridge.DocInfo{AllowMissing: true},
				},
			},
			MetadataInfo: &tfbridge.MetadataInfo{Data: info, Path: "must be non-empty"},
		}
		err = prov.ApplyAutoAliases()
		require.NoError(t, err)
		return prov
	}

	var metadata []byte
	p := provider(t, "pkg:index:R1", schema.ResourceMap{
		"pkg_r1": (&schema.Resource{}).Shim(),
	}, nil)
	metadata = ((*md.Data)(p.MetadataInfo.Data)).MarshalIndent()

	p = provider(t, "pkg:index:R1Again", schema.ResourceMap{
		"pkg_r1": (&schema.Resource{}).Shim(),
	}, metadata)
	metadata = ((*md.Data)(p.MetadataInfo.Data)).MarshalIndent()

	// Test that we don't panic
	provider(t, "", schema.ResourceMap{}, metadata)
}

type Schema struct {
	shim.Schema
	MaxItemsOne bool
	typ         shim.ValueType
	elem        any
}

func (s Schema) MaxItems() int {
	if s.MaxItemsOne {
		return 1
	}
	return 0
}

func (s Schema) Type() shim.ValueType {
	if s.typ == shim.TypeInvalid {
		return shim.TypeList
	}
	return s.typ
}

func (s Schema) Optional() bool { return true }
func (s Schema) Required() bool { return false }

func (s Schema) Elem() any { return s.elem }
