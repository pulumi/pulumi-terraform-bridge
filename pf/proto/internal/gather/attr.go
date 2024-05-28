// Copyright 2016-2024, Pulumi Corporation.
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

package gather

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
)

type _attr struct{ s *tfprotov6.SchemaAttribute }

var _ = pfutils.Attr(_attr{})

func (a _attr) IsNested() bool { return a.s.NestedType != nil } // TODO: Verify behavior

func (a _attr) Nested() map[string]pfutils.Attr {
	m := make(map[string]pfutils.Attr, len(a.s.NestedType.Attributes))
	for _, v := range a.s.NestedType.Attributes {
		m[v.Name] = _attr{v}
	}
	return m
}

func (a _attr) NestingMode() pfutils.NestingMode {
	switch a.s.NestedType.Nesting {
	case tfprotov6.SchemaObjectNestingModeList:
		return pfutils.NestingModeList
	case tfprotov6.SchemaObjectNestingModeMap:
		return pfutils.NestingModeMap
	case tfprotov6.SchemaObjectNestingModeSet:
		return pfutils.NestingModeSet
	case tfprotov6.SchemaObjectNestingModeSingle:
		return pfutils.NestingModeSingle
	default:
		return pfutils.NestingModeUnknown
	}
}

func (a _attr) HasNestedObject() bool {
	// Logic is copied from [pfutils.attrAdapter.HasNestedObject].
	switch a.NestingMode() {
	case pfutils.NestingModeList, pfutils.NestingModeMap, pfutils.NestingModeSet:
		return true
	default:
		return false
	}
}

func (a _attr) IsComputed() bool              { return a.s.Computed }
func (a _attr) IsOptional() bool              { return a.s.Optional }
func (a _attr) IsRequired() bool              { return a.s.Optional }
func (a _attr) IsSensitive() bool             { return a.s.Sensitive }
func (a _attr) GetDeprecationMessage() string { return deprecated(a.s.Deprecated) }
func (a _attr) GetDescription() string {
	if a.s.DescriptionKind == tfprotov6.StringKindPlain {
		return a.s.Description
	}
	return ""
}

func (a _attr) GetMarkdownDescription() string {
	if a.s.DescriptionKind == tfprotov6.StringKindMarkdown {
		return a.s.Description
	}
	return ""
}

func (a _attr) GetType() attr.Type {
	return _type{a.s.Type}
}
