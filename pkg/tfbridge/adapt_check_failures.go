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
	"regexp"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
)

// Underlying validators return check failures as error values. This method adapts these errors by recognizing path and
// type information to turn them into CheckFailure objects suitable for gRPC. [NewCheckFailure] makes all the
// presentation decision, while this method and the rest of the code in this file deals with adapter specifics.
func (p *Provider) adaptCheckFailures(
	ctx context.Context,
	urn resource.URN,
	isProvider bool,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
	errs []error,
) []*pulumirpc.CheckFailure {
	checkFailures := []*pulumirpc.CheckFailure{}
	for _, e := range errs {
		path, kind, reason := parseCheckError(schemaMap, schemaInfos, e)
		cf := NewCheckFailure(kind, reason, path, urn, isProvider, p.module, schemaMap, schemaInfos)
		checkFailures = append(checkFailures, &pulumirpc.CheckFailure{
			Reason:   cf.Reason,
			Property: string(cf.Property),
		})
	}
	return checkFailures
}

// Parse the TF error of a missing field:
// https://github.com/hashicorp/terraform/blob/7f5ffbfe9027c34c4ce1062a42b6e8d80b5504e0/helper/schema/schema.go#L1356
var requiredFieldRegex = regexp.MustCompile("\"(.*?)\": required field is not set")

// Similarly recognize conflicts with messages to parse their path.
var conflictsWithRegex = regexp.MustCompile("\"(.*?)\": conflicts with")

// Underlying validators return check failures as error values. This method attempts to recover type and path
// information and initially format the failure reason.
func parseCheckError(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
	err error,
) (*CheckFailurePath, CheckFailureReason, string) {
	if parts := requiredFieldRegex.FindStringSubmatch(err.Error()); len(parts) == 2 {
		name := parts[1]
		pp := NewCheckFailurePath(schemaMap, schemaInfos, name)
		return &pp, MissingKey, err.Error()
	}
	if parts := conflictsWithRegex.FindStringSubmatch(err.Error()); len(parts) == 2 {
		name := parts[1]
		pp := NewCheckFailurePath(schemaMap, schemaInfos, name)
		return &pp, MiscFailure, err.Error()
	}
	if d := (*diagnostics.ValidationError)(nil); errors.As(err, &d) {
		failType := MiscFailure
		if strings.Contains(d.Summary, "Invalid or unknown key") {
			failType = InvalidKey
		}
		pp := formatAttributePathAsPropertyPath(schemaMap, schemaInfos, d.AttributePath)
		s := d.Summary
		if d.Detail != "" {
			s += ". " + d.Detail
		}
		return pp, failType, s
	}
	// If there is no way to identify a propertyPath, still report a generic CheckFailure.
	return nil, MiscFailure, err.Error()
}

// Best effort path converter; may return nil.
func formatAttributePathAsPropertyPath(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
	attrPath cty.Path,
) *CheckFailurePath {
	steps := attrPath
	if len(steps) == 0 {
		return nil
	}
	n, ok := steps[0].(cty.GetAttrStep)
	if !ok {
		return nil
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
	return &p
}
