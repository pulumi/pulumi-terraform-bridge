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

package sdkv2

type providerOptions struct {
	diffStrategy             DiffStrategy
	planResourceChangeFilter func(string) bool
}

type providerOption func(providerOptions) (providerOptions, error)

func WithDiffStrategy(s DiffStrategy) providerOption { //nolint:revive
	return func(opts providerOptions) (providerOptions, error) {

		diffStrategyFromEnv, gotDiffStrategyFromEnv, err := ParseDiffStrategyFromEnv()
		if err != nil {
			return opts, err
		}

		if gotDiffStrategyFromEnv {
			opts.diffStrategy = diffStrategyFromEnv
			return opts, nil
		}

		opts.diffStrategy = s
		return opts, nil
	}
}

func getProviderOptions(opts []providerOption) (providerOptions, error) {
	res := providerOptions{}
	for _, o := range opts {
		var err error
		res, err = o(res)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

// Selectively opt-in resources that pass the filter to using PlanResourceChange.
func WithPlanResourceChange(filter func(string) bool) providerOption { //nolint:revive
	return func(opts providerOptions) (providerOptions, error) {
		opts.planResourceChangeFilter = filter
		return opts, nil
	}
}
