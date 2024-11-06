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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// This is a copy of the NestingMode enum in the Terraform Plugin SDK.
// It is duplicated here because the type is not exported.
type sdkV2NestingMode int

const (
	sdkV2NestingModeInvalid sdkV2NestingMode = iota
	sdkV2NestingModeSingle
	sdkV2NestingModeGroup
	sdkV2NestingModeList
	sdkV2NestingModeSet
	sdkV2NestingModeMap
)

func sdkV2NestingToShim(nesting sdkV2NestingMode) hclwrite.Nesting {
	switch nesting {
	case sdkV2NestingModeSingle:
		return hclwrite.NestingSingle
	case sdkV2NestingModeList:
		return hclwrite.NestingList
	case sdkV2NestingModeSet:
		return hclwrite.NestingSet
	default:
		contract.Failf("invalid nesting mode: %d for the SDKv2 schema", nesting)
		return hclwrite.NestingInvalid
	}
}

type hclSchemaSDKv2 map[string]*schema.Schema

var _ hclwrite.ShimHCLSchema = hclSchemaSDKv2{}

func (s hclSchemaSDKv2) GetAttributes() map[string]hclwrite.ShimHCLAttribute {
	internalMap := schema.InternalMap(s)
	coreConfigSchema := internalMap.CoreConfigSchema()
	attrMap := make(map[string]hclwrite.ShimHCLAttribute, len(s))
	for key := range coreConfigSchema.Attributes {
		attrMap[key] = hclwrite.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclSchemaSDKv2) GetBlocks() map[string]hclwrite.ShimHCLBlock {
	internalMap := schema.InternalMap(s)
	coreConfigSchema := internalMap.CoreConfigSchema()
	blockMap := make(map[string]hclwrite.ShimHCLBlock, len(coreConfigSchema.BlockTypes))
	for key, block := range coreConfigSchema.BlockTypes {
		res := s[key].Elem.(*schema.Resource)
		nesting := block.Nesting
		blockMap[key] = hclBlockSDKv2{
			hclSchemaSDKv2: hclSchemaSDKv2(res.Schema),
			nesting:        sdkV2NestingToShim(sdkV2NestingMode(nesting)),
		}
	}
	return blockMap
}

type hclBlockSDKv2 struct {
	hclSchemaSDKv2
	nesting hclwrite.Nesting
}

var _ hclwrite.ShimHCLBlock = hclBlockSDKv2{}

func (b hclBlockSDKv2) GetNestingMode() hclwrite.Nesting {
	return b.nesting
}

var _ hclwrite.ShimHCLBlock = hclBlockSDKv2{}
