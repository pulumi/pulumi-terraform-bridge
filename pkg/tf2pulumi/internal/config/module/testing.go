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
	"testing"
)

// TestTree loads a module at the given path and returns the tree as well
// as a function that should be deferred to clean up resources.
func TestTree(t *testing.T, path string) (*Tree, func()) {
	// Create a temporary directory for module storage
	dir, err := os.MkdirTemp("", "tf")
	if err != nil {
		t.Fatalf("err: %s", err)
		return nil, nil
	}

	// Load the module
	mod, err := NewTreeModule("", path)
	if err != nil {
		t.Fatalf("err: %s", err)
		return nil, nil
	}

	// Get the child modules
	s := &Storage{StorageDir: dir, Mode: GetModeGet}
	if err := mod.Load(s); err != nil {
		t.Fatalf("err: %s", err)
		return nil, nil
	}

	return mod, func() {
		os.RemoveAll(dir)
	}
}
