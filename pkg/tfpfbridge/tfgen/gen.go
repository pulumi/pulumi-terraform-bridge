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

package tfgen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/schemashim"
	realtfgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

type GenerateSchemaOptions struct {
	Context      context.Context
	ProviderInfo info.ProviderInfo
	Sink         diag.Sink
}

func GenerateSchema(opts GenerateSchemaOptions) (*schema.PackageSpec, error) {
	if opts.ProviderInfo.Name == "" {
		return nil, fmt.Errorf("opts.ProviderInfo.Name cannot be empty")
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	sink := opts.Sink
	if sink == nil {
		sink = diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		})
	}
	shimInfo := schemashim.ShimSchemaOnlyProviderInfo(ctx, opts.ProviderInfo)
	schema, err := realtfgen.GenerateSchema(shimInfo, sink)
	if err != nil {
		return nil, err
	}
	return &schema, nil
}

func MarshalSchema(schema *schema.PackageSpec) ([]byte, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema should not be nil")
	}
	bytes, err := json.MarshalIndent(*schema, "", "  ")
	if err != nil {
		return nil, err
	}
	return bytes, err
}
