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
	"path/filepath"
	"testing"

	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
)

func TestAttach(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi#15526] this will work once pulumi-yaml supports PULUMI_DEBUG_PROVIDERS")
	source := filepath.Join("..", "testdata", "basicprogram")
	bin := filepath.Join("..", "bin")
	pt := pulumitest.NewPulumiTest(t, source,
		opttest.AttachProviderBinary("testbridge", bin),
		opttest.SkipInstall())
	pt.Preview()
}
