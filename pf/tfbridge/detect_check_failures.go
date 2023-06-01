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
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"bytes"
	"fmt"

	"github.com/golang/glog"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

func (p *provider) detectCheckFailure(
	ctx context.Context,
	urn resource.URN,
	isProvider bool,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	diag *tfprotov6.Diagnostic,
) *plugin.CheckFailure {
	if diag.Attribute == nil || len(diag.Attribute.Steps()) < 1 {
		return nil
	}

	failType := miscFailure
	if diag.Summary == "Missing Configuration for Required Attribute" {
		failType = missingKey
	}
	if strings.Contains(diag.Summary, "Invalid or unknown key") {
		failType = invalidKey
	}

	pp, err := formatAttributePathAsPropertyPath(schemaMap, schemaInfos, diag.Attribute)
	if err != nil {
		glog.V(9).Infof("Ignoring path formatting error: %v", err)
		return nil
	}

	s := diag.Summary
	if diag.Detail != "" {
		s += ". " + diag.Detail
	}

	cf := formatCheckFailure(failType, s, pp, urn, isProvider, p.info.Name, schemaMap, schemaInfos)
	return &cf
}

func formatAttributePathAsPropertyPath(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	attrPath *tftypes.AttributePath,
) (propertyPath, error) {
	steps := attrPath.Steps()
	p := walk.NewSchemaPath()

	var buf bytes.Buffer
	for i, s := range steps {
		switch s := s.(type) {
		case tftypes.AttributeName:
			p = p.GetAttr(string(s))
			name, err := tfbridge.TerraformToPulumiNameAtPath(p, schemaMap, schemaInfos)
			if err != nil {
				return propertyPath{}, err
			}
			if i > 0 {
				fmt.Fprintf(&buf, ".")
			}
			fmt.Fprintf(&buf, "%s", name)
		case tftypes.ElementKeyInt:
			fmt.Fprintf(&buf, "[%d]", int64(s))
			p = p.Element()
		case tftypes.ElementKeyString:
			fmt.Fprintf(&buf, "[%q]", string(s))
			p = p.Element()
		case tftypes.ElementKeyValue:
			// Sets will be represented as lists in Pulumi; more could be done here to find the right index.
			fmt.Fprintf(&buf, "[?]")
			p = p.Element()
		default:
			contract.Failf("Unhandled match case for tftypes.AttributePathStep")
		}
	}

	return propertyPath{
		schemaPath: p,
		valuePath:  buf.String(),
	}, nil
}
