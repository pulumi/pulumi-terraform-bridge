// Copyright 2016-2022, Pulumi Corporation.
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

package module

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/go-getter"
)

// GetMode is an enum that describes how modules are loaded.
//
// GetModeLoad says that modules will not be downloaded or updated, they will
// only be loaded from the storage.
//
// GetModeGet says that modules can be initially downloaded if they don't
// exist, but otherwise to just load from the current version in storage.
//
// GetModeUpdate says that modules should be checked for updates and
// downloaded prior to loading. If there are no updates, we load the version
// from disk, otherwise we download first and then load.
type GetMode byte

const (
	GetModeNone GetMode = iota
	GetModeGet
	GetModeUpdate
)

// GetCopy is the same as Get except that it downloads a copy of the
// module represented by source.
//
// This copy will omit and dot-prefixed files (such as .git/, .hg/) and
// can't be updated on its own.
func GetCopy(dst, src string) error {
	// Create the temporary directory to do the real Get to
	tmpDir, err := os.MkdirTemp("", "tf")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpDir = filepath.Join(tmpDir, "module")

	// Get to that temporary dir
	if err := getter.Get(tmpDir, src); err != nil {
		return err
	}

	// Make sure the destination exists
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Copy to the final location
	return copyDir(dst, tmpDir)
}
