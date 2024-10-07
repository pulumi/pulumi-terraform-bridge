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

package itests

import (
	"github.com/stretchr/testify/require"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
)

func TestAttach(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a PATH setup issue where the test cannot find pulumi-resource-testbridge.exe")
	}
	source := filepath.Join("..", "testdata", "basicprogram")
	bin, err := filepath.Abs(filepath.Join("..", "bin"))
	require.NoError(t, err)
	pt := pulumitest.NewPulumiTest(t, source,
		opttest.AttachProviderBinary("testbridge", bin),
		opttest.SkipInstall())
	pt.Preview(t)
}

func TestAttachMuxed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a PATH setup issue where the test cannot find pulumi-resource-muxedrandom.exe")
	}
	source := filepath.Join("..", "testdata", "muxedbasicprogram")
	bin, err := filepath.Abs(filepath.Join("..", "bin"))
	require.NoError(t, err)
	pt := pulumitest.NewPulumiTest(t, source,
		opttest.AttachProviderBinary("muxedrandom", bin),
		opttest.SkipInstall())
	pt.Preview(t)
}
