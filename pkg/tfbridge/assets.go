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

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// AssetTranslation instructs the bridge how to translate assets into something Terraform can use.
type AssetTranslation struct {
	Kind      AssetTranslationKind   // the kind of translation to perform.
	Format    resource.ArchiveFormat // an archive format, required if this is an archive.
	HashField string                 // a field to store the hash into, if any.
}

// AssetTranslationKind may be used to choose from various source and dest translation targets.
type AssetTranslationKind int

const (
	// FileAsset turns the asset into a file on disk and passes the filename in its place.
	FileAsset AssetTranslationKind = iota
	// BytesAsset turns the asset into a []byte and passes it directly in-memory.
	BytesAsset
	// FileArchive turns the archive into a file on disk and passes the filename in its place.
	FileArchive
	// BytesArchive turns the asset into a []byte and passes that directly in-memory.
	BytesArchive
)

// Type fetches the Pulumi runtime type corresponding to values of this asset kind.
func (a *AssetTranslation) Type() string {
	switch a.Kind {
	case FileAsset, BytesAsset:
		return "Asset"
	case FileArchive, BytesArchive:
		return "Archive"
	default:
		contract.Failf("Unrecognized asset translation kind: %v", a.Kind)
		return ""
	}
}

// writeToTempFile creates a temporary file and passes it to the provided function, which will fill in the file's
// contents. Upon success, this function returns the path of the temporary file and a nil error.
func writeToTempFile(writeFunc func(w io.Writer) error) (string, error) {
	f, err := ioutil.TempFile("", "pulumi-temp-asset")
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(f)

	if err := writeFunc(f); err != nil {
		return "", err
	}

	return f.Name(), nil
}

// translateToFile translates an asset or archive to a filename. If possible, it attempts to reuse previously spilled
// assets/archives with the same identity.
func translateToFile(hash string, hasContents bool, writeFunc func(w io.Writer) error) (string, error) {
	// If possible, we want to produce a predictable filename in order to avoid spurious diffs and spilling the same
	// asset multiple times.
	memoPath := ""
	if hash != "" {
		memoPath = filepath.Join(os.TempDir(), "pulumi-asset-"+hash)
	}

	// If we have no contents, just return the file path. Note that this may be the empty string if we were also
	// missing a hash.
	if !hasContents {
		return memoPath, nil
	}

	// If we have no translation path, just write the asset to a temporary file and return.
	if memoPath == "" {
		return writeToTempFile(writeFunc)
	}

	// If the translation file already exists, assume it has the appropriate contents and return the file path.
	info, err := os.Stat(memoPath)
	if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return memoPath, nil
	}

	// Otherwise, write the asset to a temporary file, then attempt to move the temp file to the expected path.
	// If the move fails, we'll use the temp file name.
	tempName, err := writeToTempFile(writeFunc)
	if err != nil {
		return "", err
	}
	if err := os.Rename(tempName, memoPath); err != nil && !os.IsExist(err) {
		return tempName, nil
	}
	return memoPath, nil
}

// IsAsset returns true if the translation deals with an asset (rather than archive).
func (a *AssetTranslation) IsAsset() bool {
	return a.Kind == FileAsset || a.Kind == BytesAsset
}

// IsArchive returns true if the translation deals with an archive (rather than asset).
func (a *AssetTranslation) IsArchive() bool {
	return a.Kind == FileArchive || a.Kind == BytesArchive
}

// TranslateAsset translates the given asset using the directives provided by the translation info.
func (a *AssetTranslation) TranslateAsset(asset *resource.Asset) (interface{}, error) {
	contract.Assert(a.IsAsset())

	// TODO[pulumi/pulumi#153]: support HashField.

	// Now produce either a temp file or a binary blob, as requested.
	switch a.Kind {
	case FileAsset:
		path, err := translateToFile(asset.Hash, asset.HasContents(), func(w io.Writer) error {
			blob, err := asset.Read()
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(blob)

			_, err = io.Copy(w, blob)
			return err
		})
		return path, err
	case BytesAsset:
		if !asset.HasContents() {
			return []byte{}, nil
		}
		return asset.Bytes()
	default:
		contract.Failf("Unrecognized asset translation kind: %v", a.Kind)
		return nil, nil
	}
}

// TranslateArchive translates the given archive using the directives provided by the translation info.
func (a *AssetTranslation) TranslateArchive(archive *resource.Archive) (interface{}, error) {
	// TODO[pulumi/pulumi#153]: support HashField.

	// Produce either a temp file or an in-memory representation, as requested.
	format := a.Format
	if format == resource.NotArchive {
		format = resource.ZIPArchive
	}
	switch a.Kind {
	case FileArchive, FileAsset:
		path, err := translateToFile(archive.Hash, archive.HasContents(), func(w io.Writer) error {
			return archive.Archive(format, w)
		})
		return path, err
	case BytesArchive, BytesAsset:
		if !archive.HasContents() {
			return []byte{}, nil
		}
		return archive.Bytes(format)
	default:
		contract.Failf("Unrecognized asset translation kind: %v", a.Kind)
		return nil, nil
	}
}
