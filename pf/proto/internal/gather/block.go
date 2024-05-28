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

type _block struct{ s *tfprotov6.SchemaNestedBlock }

var _ = pfutils.Block(_block{})

func (b _block) NestedAttrs() map[string]pfutils.Attr {
	m := make(map[string]pfutils.Attr, len(b.s.Block.Attributes))
	for _, v := range b.s.Block.Attributes {
		m[v.Name] = _attr{v}
	}
	return m
}

func (b _block) NestedBlocks() map[string]pfutils.Block {
	m := make(map[string]pfutils.Block, len(b.s.Block.BlockTypes))
	for _, v := range b.s.Block.BlockTypes {
		m[v.TypeName] = _block{v}
	}
	return m
}

func (b _block) HasNestedObject() bool {
	// Logic is copied from [pfutils.blockAdapter.HasNestedObject].
	switch b.s.Nesting {
	case tfprotov6.SchemaNestedBlockNestingModeList,
		tfprotov6.SchemaNestedBlockNestingModeSet:
		return true
	default:
		return false
	}
}

func (b _block) GetMaxItems() int64             { return b.s.MaxItems }
func (b _block) GetMinItems() int64             { return b.s.MinItems }
func (b _block) GetDeprecationMessage() string  { return deprecated(b.s.Block.Deprecated) }
func (b _block) GetDescription() string         { return b.s.Block.Description }
func (b _block) GetMarkdownDescription() string { return b.s.Block.Description }
func (b _block) Type() attr.Type                { return _type{b.s.ValueType()} }
