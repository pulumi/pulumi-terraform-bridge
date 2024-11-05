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

// Helper code to emit Terraform HCL code to drive the Terraform CLI.
package crosstests

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
)

func sdkV2NestingToShim(nesting string) crosstestsimpl.Nesting {
	switch nesting {
	case "NestingSingle":
		return crosstestsimpl.NestingSingle
	case "NestingList":
		return crosstestsimpl.NestingList
	case "NestingSet":
		return crosstestsimpl.NestingSet
	default:
		return crosstestsimpl.NestingInvalid
	}
}

type hclSchemaSDKv2 struct {
	sch map[string]*schema.Schema
}

func NewHCLSchemaSDKv2(sch map[string]*schema.Schema) crosstestsimpl.ShimHCLSchema {
	return hclSchemaSDKv2{sch: sch}
}

var _ crosstestsimpl.ShimHCLSchema = hclSchemaSDKv2{}

func (s hclSchemaSDKv2) Attributes() map[string]crosstestsimpl.ShimHCLAttribute {
	internalMap := schema.InternalMap(s.sch)
	coreConfigSchema := internalMap.CoreConfigSchema()
	attrMap := make(map[string]crosstestsimpl.ShimHCLAttribute)
	for key, attr := range coreConfigSchema.Attributes {
		attrMap[key] = attr
	}
	return attrMap
}

func (s hclSchemaSDKv2) Blocks() map[string]crosstestsimpl.ShimHCLBlock {
	internalMap := schema.InternalMap(s.sch)
	coreConfigSchema := internalMap.CoreConfigSchema()
	blockMap := make(map[string]crosstestsimpl.ShimHCLBlock)
	for key := range coreConfigSchema.BlockTypes {
		res := s.sch[key].Elem.(*schema.Resource)
		nesting := coreConfigSchema.BlockTypes[key].Nesting.String()
		blockMap[key] = hclBlockSDKv2{
			hclSchemaSDKv2: hclSchemaSDKv2{
				sch: res.Schema,
			},
			nesting: sdkV2NestingToShim(nesting),
		}
	}
	return blockMap
}

type hclBlockSDKv2 struct {
	hclSchemaSDKv2
	nesting crosstestsimpl.Nesting
}

var _ crosstestsimpl.ShimHCLBlock = hclBlockSDKv2{}

func (b hclBlockSDKv2) GetNestingMode() crosstestsimpl.Nesting {
	return b.nesting
}

var _ crosstestsimpl.ShimHCLBlock = hclBlockSDKv2{}
