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

package tfbridge

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
)

// This function iterates over the diagnostics and replaces the names of tf config properties
// with their corresponding Pulumi names.
func replaceConfigInDiagnostics(
	config map[string]*tfbridge.SchemaInfo,
	schema shim.SchemaMap,
	diags []*tfprotov6.Diagnostic,
) []*tfprotov6.Diagnostic {
	result := make([]*tfprotov6.Diagnostic, len(diags))
	copy(result, diags)
	for i, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			summary, summaryErr := tfbridge.ReplaceConfigProperties(d.Summary, config, schema)
			if summaryErr != nil {
				newDiag := tfprotov6.Diagnostic{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Property replacement error",
					Detail:   summaryErr.Error(),
				}
				result = append(result, &newDiag)
			} else {
				result[i].Summary = summary
			}

			detail, detailErr := tfbridge.ReplaceConfigProperties(d.Detail, config, schema)
			if detailErr != nil {
				newDiag := tfprotov6.Diagnostic{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Property replacement error",
					Detail:   detailErr.Error(),
				}
				result = append(result, &newDiag)
			} else {
				result[i].Detail = detail
			}
		}
	}
	return result
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *provider) ConfigureWithContext(ctx context.Context, inputs resource.PropertyMap) error {
	ctx = p.initLogging(ctx, p.logSink, "")

	configureSpan, ctx := opentracing.StartSpanFromContext(ctx, "Configure",
		opentracing.Tag{Key: "framework", Value: "plugin-framework"},
		opentracing.Tag{Key: "provider", Value: p.info.Name},
		opentracing.Tag{Key: "version", Value: p.version.String()})
	defer configureSpan.Finish()

	p.lastKnownProviderConfig = inputs

	config, err := convert.EncodePropertyMapToDynamic(p.configEncoder, p.configType, inputs)
	if err != nil {
		return fmt.Errorf("cannot encode provider configuration to call ConfigureProvider: %w", err)
	}

	req := &tfprotov6.ConfigureProviderRequest{
		Config:           config,
		TerraformVersion: "pulumi-terraform-bridge",
	}

	resp, err := p.tfServer.ConfigureProvider(ctx, req)
	if err != nil {
		return fmt.Errorf("error calling ConfigureProvider: %w", err)
	}

	resp.Diagnostics = replaceConfigInDiagnostics(p.info.Config, p.info.P.Schema(), resp.Diagnostics)
	return p.processDiagnostics(resp.Diagnostics)
}
