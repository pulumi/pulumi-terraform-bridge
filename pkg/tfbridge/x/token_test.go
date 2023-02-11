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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokensSingleModule(t *testing.T) {
	info := tfbridge.ProviderInfo{
		P: Provider{
			resources: map[string]struct{}{
				"foo_fizz_buzz":       {},
				"foo_bar_hello_world": {},
				"foo_bar":             {},
			},
			datasources: map[string]struct{}{
				"foo_source1":             {},
				"foo_very_special_source": {},
			},
		},
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
		P: Provider{
			resources: map[string]struct{}{
				"cs101_fizz_buzz_one_five": {},
				"cs101_fizz_three":         {},
				"cs101_fizz_three_six":     {},
				"cs101_buzz_five":          {},
				"cs101_buzz_ten":           {},
				"cs101_game":               {},
			},
		},
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
		P: Provider{
			resources: map[string]struct{}{
				"cs101_fizz_buzz_one_five": {},
				"cs101_fizz_three":         {},
				"cs101_fizz_three_six":     {},
				"cs101_buzz_five":          {},
				"cs101_buzz_ten":           {},
				"cs101_game":               {},
			},
		},
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
		P: Provider{
			resources: map[string]struct{}{
				"cs101_one_five":  {},
				"cs101_three":     {},
				"cs101_three_six": {},
			},
		},
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
		opts            *tfbridge.InferredModulesOpts
	}{
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
			opts: &tfbridge.InferredModulesOpts{
				MinimumModuleSize: 2,
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
			opts: &tfbridge.InferredModulesOpts{
				TfPkgPrefix:          "pkg_",
				MinimumModuleSize:    3,
				MimimumSubmoduleSize: 4,
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
			opts: &tfbridge.InferredModulesOpts{
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
			opts: &tfbridge.InferredModulesOpts{
				MinimumModuleSize: 3,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			resources := map[string]struct{}{}
			for k := range tt.resourceMapping {
				resources[k] = struct{}{}
			}
			info := &tfbridge.ProviderInfo{
				P: Provider{
					resources: resources,
				},
			}

			strategy, err := tfbridge.TokensInferredModules(info,
				func(module, name string) (string, error) { return module + ":" + name, nil },
				tt.opts)
			require.NoError(t, err)
			err = info.ComputeDefaults(strategy)
			require.NoError(t, err)

			mapping := map[string]string{}
			for k, v := range info.Resources {
				mapping[k] = v.Tok.String()
			}
			assert.Equal(t, tt.resourceMapping, mapping)
		})
	}
}

type Provider struct {
	util.UnimplementedProvider

	// We are only concerned with tokens, so that's all we support
	datasources map[string]struct{}
	resources   map[string]struct{}
}

func (p Provider) ResourcesMap() shim.ResourceMap   { return ResourceMap{p.resources} }
func (p Provider) DataSourcesMap() shim.ResourceMap { return ResourceMap{p.datasources} }

type ResourceMap struct{ m map[string]struct{} }
type Resource struct{ t string }

func (m ResourceMap) Len() int                     { return len(m.m) }
func (m ResourceMap) Get(key string) shim.Resource { return Resource{key} }
func (m ResourceMap) GetOk(key string) (shim.Resource, bool) {
	_, ok := m.m[key]
	if !ok {
		return nil, false
	}
	return Resource{key}, true
}
func (m ResourceMap) Range(each func(key string, value shim.Resource) bool) {
	for k := range m.m {
		each(k, Resource{k})
	}
}
func (m ResourceMap) Set(key string, value shim.Resource) {
	m.m[key] = struct{}{}
}

func (r Resource) Schema() shim.SchemaMap          { panic("unimplemented") }
func (r Resource) SchemaVersion() int              { panic("unimplemented") }
func (r Resource) Importer() shim.ImportFunc       { panic("unimplemented") }
func (r Resource) DeprecationMessage() string      { panic("unimplemented") }
func (r Resource) Timeouts() *shim.ResourceTimeout { panic("unimplemented") }
func (r Resource) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
	panic("unimplemented")
}
func (r Resource) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("unimplemented")
}
