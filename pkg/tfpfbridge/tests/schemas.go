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
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tfgen"
)

func genSchemaBytes(t *testing.T, info info.ProviderInfo) []byte {
	packageSpec, err := tfgen.GenerateSchema(tfgen.GenerateSchemaOptions{
		ProviderInfo: info,
		Sink:         testSink(t),
	})
	require.NoError(t, err)
	bytes, err := tfgen.MarshalSchema(packageSpec)
	require.NoError(t, err)
	t.Logf("SCHEMA:\n%v", string(bytes))
	return bytes
}

func genRandomSchemaBytes(t *testing.T) []byte {
	info := testprovider.RandomProvider()
	return genSchemaBytes(t, info)
}

func genTestBridgeSchemaBytes(t *testing.T) []byte {
	info := testprovider.SyntheticTestBridgeProvider()
	return genSchemaBytes(t, info)
}

func testSink(t *testing.T) diag.Sink {
	var stdout, stderr bytes.Buffer

	testSink := diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	t.Cleanup(func() {
		t.Logf("%s\n", stdout.String())
		t.Logf("%s\n", stderr.String())
	})

	return testSink
}
