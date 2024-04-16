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

// This file provides backward compatibility for moving [info.AssetTranslation] and
// [info.AssetTranslationKind] to [info].

import "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"

// AssetTranslation instructs the bridge how to translate assets into something Terraform can use.
type AssetTranslation = info.AssetTranslation

// AssetTranslationKind may be used to choose from various source and dest translation targets.
type AssetTranslationKind = info.AssetTranslationKind

const (
	// FileAsset turns the asset into a file on disk and passes the filename in its place.
	FileAsset = info.FileAsset
	// BytesAsset turns the asset into a []byte and passes it directly in-memory.
	BytesAsset = info.BytesAsset
	// FileArchive turns the archive into a file on disk and passes the filename in its place.
	FileArchive = info.FileArchive
	// BytesArchive turns the asset into a []byte and passes that directly in-memory.
	BytesArchive = info.BytesArchive
)
