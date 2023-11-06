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

package tfbridge

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A KV store persisted between `tfgen` and a running provider.
//
// The store is read-write when the schema is being generated, and is persisted to disk
// with schema.json. During normal provider operation (pulumi-resource-${PKG}), the store
// if not persisted (making it effectively read-only).
type ProviderMetadata = *metadata.Data

// Information necessary to persist and use provider level metadata.
type MetadataInfo struct {
	// The path (relative to schema.json) of the metadata file.
	Path string
	// The parsed metadata.
	Data ProviderMetadata
}

// Describe a metadata file called `bridge-metadata.json`.
//
// `bytes` is the embedded metadata file.
func NewProviderMetadata(bytes []byte) *MetadataInfo {
	parsed, err := metadata.New(bytes)
	// We assert instead of returning an (MetadataInfo, error) because we are
	// validating compile time embedded data.
	//
	// The error could never be handled, because it signals that invalid data was
	// `go:embed`ed.
	contract.AssertNoErrorf(err, "This always signals an error at compile time.")

	info := &MetadataInfo{"bridge-metadata.json", ProviderMetadata(parsed)}
	info.assertValid()
	return info
}

func (info *MetadataInfo) assertValid() {
	contract.Assertf(info != nil,
		"Attempting to use provider metadata without setting ProviderInfo.MetadataInfo")

	// We assert instead of returning an (MetadataInfo, error) because path should be
	// a string constant, the "tfgen time" location from which bytes was
	// extracted. This error is irrecoverable and needs to be fixed at compile time.
	contract.Assertf(info.Path != "", "Path must be non-empty")

}
