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

// Re-export metadata in an opaque format.
package metadata

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// A store persisted between `tfgen` and a running provider.
type Provider *metadata.Data

// Create a new ProviderMetadata from a persisted byte slice.
func New(data []byte) (Provider, error) {
	parsed, err := metadata.New(data)
	if err != nil {
		return nil, err
	}
	return Provider(parsed), nil
}

func Marshal(p Provider) []byte {
	return (*metadata.Data)(p).Marshal()
}
