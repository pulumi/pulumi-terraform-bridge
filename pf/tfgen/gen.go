// Copyright 2016-2023, Pulumi Corporation.
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

package tfgen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	sdkbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	realtfgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

type GenerateSchemaOptions struct {
	ProviderInfo    sdkbridge.ProviderInfo
	DiagnosticsSink diag.Sink
}

type GenerateSchemaResult struct {
	ProviderMetadata tfbridge.ProviderMetadata
}

// Generates the Pulumi Package Schema and bridge-specific metadata. Most users do not need to call this directly but
// instead use Main to build a build-time helper CLI tool.
func GenerateSchema(ctx context.Context, opts GenerateSchemaOptions) (*GenerateSchemaResult, error) {
	if opts.ProviderInfo.Name == "" {
		return nil, fmt.Errorf("opts.ProviderInfo.Name cannot be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sink := opts.DiagnosticsSink
	if sink == nil {
		sink = diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		})
	}

	generated, err := realtfgen.GenerateSchemaWithOptions(realtfgen.GenerateSchemaOptions{
		ProviderInfo:    opts.ProviderInfo,
		DiagnosticsSink: sink,
	})

	if err != nil {
		return nil, err
	}

	schema, err := json.Marshal(generated.PackageSpec)
	if err != nil {
		return nil, err
	}

	return &GenerateSchemaResult{
		ProviderMetadata: tfbridge.ProviderMetadata{
			PackageSchema: schema,
		},
	}, nil
}
