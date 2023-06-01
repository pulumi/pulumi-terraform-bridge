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
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/hashicorp/go-cty/cty"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
)

func (p *Provider) detectCheckFailures(
	ctx context.Context,
	urn resource.URN,
	isProvider bool,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
	errs []error,
) []*pulumirpc.CheckFailure {
	checkFailures := []*pulumirpc.CheckFailure{}
	for _, e := range errs {
		cf := p.detectCheckFailure(ctx, urn, isProvider, schemaMap, schemaInfos, e)
		if cf != nil {
			checkFailures = append(checkFailures, &pulumirpc.CheckFailure{
				Reason:   cf.Reason,
				Property: string(cf.Property),
			})
		} else {
			msg := fmt.Sprintf("%v", e)
			logErr := p.host.Log(ctx, diag.Warning, urn, msg)
			if logErr != nil {
				glog.V(9).Infof("Failed to log to a warning to the engine: %v. Warn: %s", logErr, msg)
			}
		}
	}
	return checkFailures
}

// Parse the TF error of a missing field:
// https://github.com/hashicorp/terraform/blob/7f5ffbfe9027c34c4ce1062a42b6e8d80b5504e0/helper/schema/schema.go#L1356
var requiredFieldRegex = regexp.MustCompile("\"(.*?)\": required field is not set")

var conflictsWithRegex = regexp.MustCompile("\"(.*?)\": conflicts with")

func (p *Provider) detectCheckFailure(
	ctx context.Context,
	urn resource.URN,
	isProvider bool,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
	err error,
) *plugin.CheckFailure {
	if parts := requiredFieldRegex.FindStringSubmatch(err.Error()); len(parts) == 2 {
		name := parts[1]
		pp := NewCheckFailurePath(schemaMap, schemaInfos, name)
		f := NewCheckFailure(MissingKey, err.Error(), pp, urn, isProvider, p.info.Name, schemaMap, schemaInfos)
		return &f
	}
	if parts := conflictsWithRegex.FindStringSubmatch(err.Error()); len(parts) == 2 {
		name := parts[1]
		pp := NewCheckFailurePath(schemaMap, schemaInfos, name)
		f := NewCheckFailure(MiscFailure, err.Error(), pp, urn, isProvider, p.info.Name, schemaMap, schemaInfos)
		return &f
	}
	var d *diagnostics.ValidationError
	if !errors.As(err, &d) {
		return nil
	}
	if d.AttributePath == nil || len(d.AttributePath) < 1 {
		return nil
	}
	pp, err := formatAttributePathAsPropertyPath(schemaMap, schemaInfos, d.AttributePath)
	if err != nil {
		glog.V(9).Infof("Ignoring path formatting error: %v", err)
		return nil
	}
	failType := MiscFailure
	if strings.Contains(d.Summary, "Invalid or unknown key") {
		failType = InvalidKey
	}
	s := d.Summary
	if d.Detail != "" {
		s += ". " + d.Detail
	}
	cf := NewCheckFailure(failType, s, pp, urn, isProvider, p.info.Name, schemaMap, schemaInfos)
	return &cf
}

func formatAttributePathAsPropertyPath(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
	attrPath cty.Path,
) (ret CheckFailurePath, finalErr error) {
	steps := attrPath
	if len(steps) == 0 {
		return ret, fmt.Errorf("Expected a path with at least 1 step")
	}
	n, ok := steps[0].(cty.GetAttrStep)
	if !ok {
		return ret, fmt.Errorf("Expected a path that starts with an AttributeName step")
	}
	p := NewCheckFailurePath(schemaMap, schemaInfos, n.Name)
	for _, s := range steps[1:] {
		switch s := s.(type) {
		case cty.GetAttrStep:
			p = p.Attribute(s.Name)
		case cty.IndexStep:
			key := s.Key
			switch key.Type() {
			case cty.String:
				p = p.MapElement(key.AsString())
			case cty.Number:
				i, _ := key.AsBigFloat().Int64()
				p = p.ListElement(i)
			default:
				p = p.SetElement()
			}
		default:
			contract.Failf("Unhandled match case for cty.Path")
		}
	}
	return p, nil
}
