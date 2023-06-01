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
	"strings"

	"github.com/golang/glog"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
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

	failType := tfbridge.MiscFailure
	if diag.Summary == "Missing Configuration for Required Attribute" {
		failType = tfbridge.MissingKey
	}
	if strings.Contains(diag.Summary, "Invalid or unknown key") {
		failType = tfbridge.InvalidKey
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

	cf := tfbridge.NewCheckFailure(failType, s, pp, urn, isProvider, p.info.Name, schemaMap, schemaInfos)
	return &cf
}

func formatAttributePathAsPropertyPath(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	attrPath *tftypes.AttributePath,
) (ret tfbridge.CheckFailurePath, finalErr error) {
	steps := attrPath.Steps()
	if len(steps) == 0 {
		return ret, fmt.Errorf("Expected a path with at least 1 step")
	}
	n, ok := steps[0].(tftypes.AttributeName)
	if !ok {
		return ret, fmt.Errorf("Expected a path that starts with an AttributeName step")
	}
	p := tfbridge.NewCheckFailurePath(schemaMap, schemaInfos, string(n))
	for _, s := range steps[1:] {

		switch s := s.(type) {
		case tftypes.AttributeName:
			p = p.Attribute(string(s))
		case tftypes.ElementKeyInt:
			p = p.ListElement(int64(s))
		case tftypes.ElementKeyString:
			p = p.MapElement(string(s))
		case tftypes.ElementKeyValue:
			p = p.SetElement()
		default:
			contract.Failf("Unhandled match case for tftypes.AttributePathStep")
		}

	}
	return p, nil
}
