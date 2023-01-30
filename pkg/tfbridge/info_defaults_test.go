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
	//"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestSetDefaultsSingleModule(t *testing.T) {
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

	err := info.SetDefaults(tfbridge.SuggestTokens("foo", tfbridge.SingleModule("foo_", "index")))
	assert.NoError(t, err)

	expectedResources := map[string]*tfbridge.ResourceInfo{
		"foo_fizz_buzz":       {Tok: "foo:index/fizzBuzz:FizzBuzz"},
		"foo_bar_hello_world": {Tok: "foo:index/barHelloWorld:BarHelloWorld"},
		"foo_bar":             {Tok: "foo:index/bar:Bar"},
	}
	expectedDatasources := map[string]*tfbridge.DataSourceInfo{
		"foo_source1":             {Tok: "foo:index/source1:getSource1"},
		"foo_very_special_source": {Tok: "foo:index/verySpecialSource:getVerySpecialSource"},
	}

	assert.Equal(t, expectedResources, info.Resources)
	assert.Equal(t, expectedDatasources, info.DataSources)

	// Now test that overrides still work
	info.Resources = map[string]*tfbridge.ResourceInfo{
		"foo_bar_hello_world": {Tok: "foo:index:BarHelloPulumi"},
	}
	err = info.SetDefaults(tfbridge.SuggestTokens("foo", tfbridge.SingleModule("foo_", "index")))
	assert.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"foo_fizz_buzz":       {Tok: "foo:index/fizzBuzz:FizzBuzz"},
		"foo_bar_hello_world": {Tok: "foo:index:BarHelloPulumi"},
		"foo_bar":             {Tok: "foo:index/bar:Bar"},
	}, info.Resources)
}

func TestSetDefaultsKnownModules(t *testing.T) {
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

	modParser := tfbridge.KnownModules("cs101_", "index", []string{
		"fizz_", "buzz_", "fizz_buzz_",
	})

	err := info.SetDefaults(tfbridge.SuggestTokens("cs101", modParser))
	assert.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz/oneFive:OneFive"},
		"cs101_fizz_three":         {Tok: "cs101:fizz/three:Three"},
		"cs101_fizz_three_six":     {Tok: "cs101:fizz/threeSix:ThreeSix"},
		"cs101_buzz_five":          {Tok: "cs101:buzz/five:Five"},
		"cs101_buzz_ten":           {Tok: "cs101:buzz/ten:Ten"},
		"cs101_game":               {Tok: "cs101:index/game:Game"},
	}, info.Resources)
}

func TestSetDefaultsUnmappable(t *testing.T) {
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

	modParser := tfbridge.KnownModules("cs101_", "index", []string{
		"fizz_", "buzz_", "fizz_buzz_",
	})

	err := info.SetDefaults(tfbridge.SuggestTokens("cs101", modParser).Unmappable("five"))
	assert.Error(t, err)

	// strategy := strategy.Unmappable("five")
	// err := info.ComputeDefaultResources(strategy)
	// assert.ErrorContains(t, err, "contains unmapable sub-string")

	// Override the unmappable resources
	info.Resources = map[string]*tfbridge.ResourceInfo{
		// When "five" comes after another number, we print it as "5"
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz:One5"},
		"cs101_buzz_five":          {Tok: "cs101:buzz:Five"},
	}

	err = info.SetDefaults(tfbridge.SuggestTokens("cs101", modParser).Unmappable("five"))
	assert.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.ResourceInfo{
		"cs101_fizz_buzz_one_five": {Tok: "cs101:fizzBuzz:One5"},
		"cs101_fizz_three":         {Tok: "cs101:fizz/three:Three"},
		"cs101_fizz_three_six":     {Tok: "cs101:fizz/threeSix:ThreeSix"},
		"cs101_buzz_five":          {Tok: "cs101:buzz:Five"},
		"cs101_buzz_ten":           {Tok: "cs101:buzz/ten:Ten"},
		"cs101_game":               {Tok: "cs101:index/game:Game"},
	}, info.Resources)
}
