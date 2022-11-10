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

package tfbridgetests

import (
	"testing"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge"
	testutils "github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testprovider"
)

// These tests replay gRPC logs from a well-behaved test program in testdatagen/genrandom to verify
// bridged provider methods. This covers Check, Diff, Create, Delete. Random provider currently
// never plans updates so this test does not cover Updates.
//
// See testdatagen/genrandom/generate.sh for regenerating the test data.
func TestGenRandom(t *testing.T) {
	traces := []string{
		// TODO enable once Configure replay is implemented.
		// "testdata/genrandom/random-delete-preview.json",

		"testdata/genrandom/random-delete-update.json",
		"testdata/genrandom/random-empty-preview.json",
		"testdata/genrandom/random-empty-update.json",
		"testdata/genrandom/random-initial-preview.json",
		"testdata/genrandom/random-initial-update.json",
		"testdata/genrandom/random-replace-preview.json",
		"testdata/genrandom/random-replace-update.json",
	}
	schema := genRandomSchemaBytes(t)

	for _, trace := range traces {
		trace := trace

		t.Run(trace, func(t *testing.T) {
			p := testprovider.RandomProvider()
			server := tfbridge.NewProviderServer(p, schema)
			testutils.ReplayTraceFile(t, server, trace)
		})
	}
}
