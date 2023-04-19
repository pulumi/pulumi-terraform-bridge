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

package x

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
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
	}

	makeToken := func(module, name string) (string, error) {
		return fmt.Sprintf("foo:%s:%s", module, name), nil
	}
	opts := TokensSingleModule("foo_", "index", makeToken)
	err := ComputeDefaults(&info, opts)
	require.NoError(t, err)

	expectedResources := map[string]*tfbridge.ResourceInfo{
		"foo_fizz_buzz":       {Tok: "foo:index:FizzBuzz"},
		"foo_bar_hello_world": {Tok: "foo:index:BarHelloWorld"},
		"foo_bar":             {Tok: "foo:index:Bar"},
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
	err = ComputeDefaults(&info, DefaultStrategy{
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

	err := ComputeDefaults(&info, DefaultStrategy{
		Resource: TokensKnownModules("cs101_", "index", []string{
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

	strategy := TokensKnownModules("cs101_", "index", []string{
		"fizz_", "buzz_", "fizz_buzz_",
	}, func(module, name string) (string, error) {
		return fmt.Sprintf("cs101:%s:%s", module, name), nil
	})
	strategy = strategy.Unmappable("five", "SomeGoodReason")
	err := ComputeDefaults(&info, strategy)
	assert.ErrorContains(t, err, "SomeGoodReason")

	// Override the unmappable resources
	info.Resources = map[string]*tfbridge.ResourceInfo{
		// When "five" comes after another number, we print it as "5"
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz:One5"},
		"cs101_buzz_five":          {Tok: "cs101:buzz:Five"},
	}
	err = ComputeDefaults(&info, strategy)
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
	err := ComputeDefaults(&info, TokensSingleModule("cs101_", "index_", MakeStandardToken("cs101")))
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
		opts            *InferredModulesOpts
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
			opts: &InferredModulesOpts{
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
			opts: &InferredModulesOpts{
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
			opts: &InferredModulesOpts{
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
			opts: &InferredModulesOpts{
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
			opts: &InferredModulesOpts{
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

			strategy, err := TokensInferredModules(info,
				func(module, name string) (string, error) { return module + ":" + name, nil },
				tt.opts)
			require.NoError(t, err)
			err = ComputeDefaults(info, strategy)
			require.NoError(t, err)

			mapping := map[string]string{}
			for k, v := range info.Resources {
				mapping[k] = v.Tok.String()
			}
			assert.Equal(t, tt.resourceMapping, mapping)
		})
	}
}

func TestTokenAliasing(t *testing.T) {
	provider := func() *tfbridge.ProviderInfo {
		return &tfbridge.ProviderInfo{
			P: (&schema.Provider{
				ResourcesMap: schema.ResourceMap{
					"pkg_mod1_r1": nil,
					"pkg_mod1_r2": nil,
					"pkg_mod2_r1": nil,
				},
			}).Shim(),
		}
	}
	simple := provider()

	metadata, err := metadata.New(nil)
	require.NoError(t, err)

	err = ComputeDefaults(simple, TokensSingleModule("pkg_", "index", MakeStandardToken("pkg")))
	require.NoError(t, err)

	err = AutoAliasing(simple, metadata)
	require.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"pkg_mod1_r1": {Tok: "pkg:index/mod1R1:Mod1R1"},
		"pkg_mod1_r2": {Tok: "pkg:index/mod1R2:Mod1R2"},
		"pkg_mod2_r1": {Tok: "pkg:index/mod2R1:Mod2R1"},
	}, simple.Resources)

	modules := provider()
	modules.Version = "1.0.0"

	knownModules := TokensKnownModules("pkg_", "",
		[]string{"mod1", "mod2"}, MakeStandardToken("pkg"))

	err = ComputeDefaults(modules, knownModules)
	require.NoError(t, err)

	err = AutoAliasing(modules, metadata)
	require.NoError(t, err)

	hist2 := md.Clone(metadata)
	ref := func(s string) *string { return &s }
	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
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

	modules2 := provider()
	modules2.Version = "1.0.0"

	err = ComputeDefaults(modules2, knownModules)
	require.NoError(t, err)

	err = AutoAliasing(modules2, metadata)
	require.NoError(t, err)

	hist3 := md.Clone(metadata)
	assert.Equal(t, string(hist2.Marshal()), string(hist3.Marshal()),
		"No changes should imply no change in history")
	assert.Equal(t, modules, modules2)

	modules3 := provider()
	modules3.Version = "100.0.0"

	err = ComputeDefaults(modules3, knownModules)
	require.NoError(t, err)

	err = AutoAliasing(modules3, metadata)
	require.NoError(t, err)

	// All hard aliases should be removed on a major version upgrade
	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
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
	}, modules3.Resources)

	// A provider with no version should assume the most recent major
	// version in history – in this case, all aliases should be kept
	modules4 := provider()

	err = ComputeDefaults(modules4, knownModules)
	require.NoError(t, err)

	err = AutoAliasing(modules4, metadata)
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
					}}).Shim(),
				},
			}).Shim(),
		}
		err := ComputeDefaults(prov, TokensSingleModule("pkg_", "index", MakeStandardToken("pkg")))
		require.NoError(t, err)
		return prov
	}
	info := provider(true, false)
	metadata, err := metadata.New(nil)
	require.NoError(t, err)

	// Save current state into metadata
	err = AutoAliasing(info, metadata)
	require.NoError(t, err)

	v := string(metadata.Marshal())
	expected := `{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemOne": true
                    },
                    "f2": {
                        "maxItemOne": false
                    }
                }
            }
        },
        "datasources": {}
    }
}`
	assert.Equal(t, expected, v)

	info = provider(false, true)

	// Apply metadata back into the provider
	err = AutoAliasing(info, metadata)
	require.NoError(t, err)

	assert.True(t, *info.Resources["pkg_r1"].Fields["f1"].MaxItemsOne)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f2"].MaxItemsOne)
	assert.Equal(t, expected, string(metadata.Marshal()))

	// Apply metadata back into the provider again, making sure there isn't a diff
	err = AutoAliasing(info, metadata)
	require.NoError(t, err)

	assert.True(t, *info.Resources["pkg_r1"].Fields["f1"].MaxItemsOne)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f2"].MaxItemsOne)
	assert.Equal(t, expected, string(metadata.Marshal()))

	// Validate that overrides work

	info = provider(true, false)
	info.Resources["pkg_r1"].Fields = map[string]*tfbridge.SchemaInfo{
		"f1": {MaxItemsOne: tfbridge.False()},
	}

	err = AutoAliasing(info, metadata)
	require.NoError(t, err)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f1"].MaxItemsOne)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f2"].MaxItemsOne)
	assert.Equal(t, `{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemOne": false
                    },
                    "f2": {
                        "maxItemOne": false
                    }
                }
            }
        },
        "datasources": {}
    }
}`, string(metadata.Marshal()))
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
		err := ComputeDefaults(prov, TokensSingleModule("pkg_", "index", MakeStandardToken("pkg")))
		require.NoError(t, err)
		return prov
	}
	info := provider(true, false)
	metadata, err := metadata.New(nil)
	require.NoError(t, err)

	// Save current state into metadata
	err = AutoAliasing(info, metadata)
	require.NoError(t, err)

	v := string(metadata.Marshal())
	expected := `{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemOne": true
                    },
                    "f2": {
                        "maxItemOne": false
                    }
                }
            }
        },
        "datasources": {}
    }
}`
	assert.Equal(t, expected, v)

	info = provider(false, true)

	// Apply metadata back into the provider
	info.Version = "1.0.0" // New major version
	err = AutoAliasing(info, metadata)
	require.NoError(t, err)

	assert.Nil(t, info.Resources["pkg_r1"].Fields["f1"].MaxItemsOne)
	assert.Nil(t, info.Resources["pkg_r1"].Fields["f2"].MaxItemsOne)
	assert.Equal(t, `{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "majorVersion": 1,
                "fields": {
                    "f1": {
                        "maxItemOne": false
                    },
                    "f2": {
                        "maxItemOne": true
                    }
                }
            }
        },
        "datasources": {}
    }
}`, string(metadata.Marshal()))

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
		err := ComputeDefaults(prov, TokensSingleModule("pkg_", "index", MakeStandardToken("pkg")))
		require.NoError(t, err)
		return prov
	}
	info := provider(true, false)
	metadata, err := metadata.New(nil)
	require.NoError(t, err)

	// Save current state into metadata
	err = AutoAliasing(info, metadata)
	require.NoError(t, err)

	v := string(metadata.Marshal())
	expected := `{
    "auto-aliasing": {
        "resources": {
            "pkg_r1": {
                "current": "pkg:index/r1:R1",
                "fields": {
                    "f1": {
                        "maxItemOne": false
                    },
                    "f2": {
                        "maxItemOne": false,
                        "fields": {
                            "n1": {
                                "maxItemOne": true
                            },
                            "n2": {
                                "maxItemOne": false
                            }
                        }
                    }
                }
            }
        },
        "datasources": {}
    }
}`
	assert.Equal(t, expected, v)

	// Apply the saved metadata to a new provider
	info = provider(false, true)
	err = AutoAliasing(info, metadata)
	require.NoError(t, err)

	assert.Equal(t, expected, string(metadata.Marshal()))
	assert.True(t, *info.Resources["pkg_r1"].Fields["f2"].Fields["n1"].MaxItemsOne)
	assert.False(t, *info.Resources["pkg_r1"].Fields["f2"].Fields["n2"].MaxItemsOne)
}

type Schema struct {
	shim.Schema
	MaxItemsOne bool
	elem        any
}

func (s Schema) MaxItems() int {
	if s.MaxItemsOne {
		return 1
	}
	return 0
}

func (s Schema) Type() shim.ValueType {
	return shim.TypeList
}

func (s Schema) Elem() any { return s.elem }
