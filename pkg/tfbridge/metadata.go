// Copyright 2016-2024, Pulumi Corporation.
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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/internal/metadatakeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// A KV store persisted between `tfgen` and a running provider.
//
// The store is read-write when the schema is being generated, and is persisted to disk
// with schema.json. During normal provider operation (pulumi-resource-${PKG}), the store
// if not persisted (making it effectively read-only).
type ProviderMetadata = info.ProviderMetadata

// Information necessary to persist and use provider level metadata.
type MetadataInfo = info.Metadata

// Describe a metadata file called `bridge-metadata.json`.
//
// `bytes` is the embedded metadata file.
func NewProviderMetadata(bytes []byte) *MetadataInfo { return info.NewProviderMetadata(bytes) }

var declaredRuntimeMetadata = struct {
	keys map[string]struct{}
	m    sync.Mutex
}{keys: map[string]struct{}{
	autoSettingsKey:                          {},
	metadatakeys.DefaultResourceSchemaFixups: {},
	"mux":                                    {},
}}

func declareRuntimeMetadata(label string) {
	declaredRuntimeMetadata.m.Lock()
	defer declaredRuntimeMetadata.m.Unlock()
	declaredRuntimeMetadata.keys[label] = struct{}{}
}

// ExtractRuntimeMetadata trims provider metadata to the keys needed by runtime
// provider startup.
//
// The returned metadata includes an internal runtime marker in the blob itself.
// Runtime consumers intentionally key off that marker, not MetadataInfo.Path,
// because downstream providers may embed these bytes and still load them through
// NewProviderMetadata.
func ExtractRuntimeMetadata(metadataInfo *MetadataInfo) *MetadataInfo {
	data, _ := metadata.New(nil)
	declaredRuntimeMetadata.m.Lock()
	defer declaredRuntimeMetadata.m.Unlock()
	for k := range declaredRuntimeMetadata.keys {
		metadata.CloneKey(k, metadataInfo.Data, data)
	}
	err := metadata.Set(data, metadatakeys.RuntimeMetadata, true)
	contract.AssertNoErrorf(err, "failed to write runtime metadata marker")

	return &MetadataInfo{
		Path: "runtime-bridge-metadata.json",
		Data: ProviderMetadata(data),
	}
}
