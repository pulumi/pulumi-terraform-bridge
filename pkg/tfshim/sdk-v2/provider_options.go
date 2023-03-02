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
	diffStrategy DiffStrategy
}

type providerOption func(providerOptions) (providerOptions, error)

func WithDiffStrategy(s DiffStrategy) providerOption {
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
