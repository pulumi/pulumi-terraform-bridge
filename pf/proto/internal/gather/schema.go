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
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
)

type schema struct{ s *tfprotov6.Schema }

var _ = pfutils.Schema(schema{})

func (s schema) ApplyTerraform5AttributePathStep(tftypes.AttributePathStep) (interface{}, error) {
	panic("UNIMPLIMENTED")
}

func (s schema) Type() attr.Type { return _type{s.s.ValueType()} }

func (s schema) Attrs() map[string]pfutils.Attr {
	attrs := make(map[string]pfutils.Attr, len(s.s.Block.Attributes))
	for _, v := range s.s.Block.Attributes {
		attrs[v.Name] = _attr{v}
	}
	return attrs
}

func (s schema) Blocks() map[string]pfutils.Block {
	block := make(map[string]pfutils.Block, len(s.s.Block.BlockTypes))
	for _, v := range s.s.Block.BlockTypes {
		block[v.TypeName] = _block{v}
	}
	return block

}

func (s schema) DeprecationMessage() string { return deprecated(s.s.Block.Deprecated) }

func (s schema) AttributeAtPath(context.Context, path.Path) (pfutils.Attr, diag.Diagnostics) {
	panic("UNIMPLIMENTED")
}

func (s schema) ResourceSchemaVersion() int64 { return s.s.Version }
