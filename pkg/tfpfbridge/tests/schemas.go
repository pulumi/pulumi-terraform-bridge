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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	tfpf "github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tfgen"
)

func genMetadata(t *testing.T, info tfpf.ProviderInfo) tfpf.ProviderMetadata {
	generated, err := tfgen.GenerateSchema(context.Background(), tfgen.GenerateSchemaOptions{
		ProviderInfo:    info,
		DiagnosticsSink: testSink(t),
	})
	require.NoError(t, err)
	return generated.ProviderMetadata
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
