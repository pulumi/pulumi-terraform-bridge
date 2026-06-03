// Copyright 2016-2026, Pulumi Corporation.
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

package pfutils

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
)

type lazySchema struct {
	kind   string
	tfName runtypes.TypeName
	load   func() (Schema, error)

	once   sync.Once
	schema Schema
	err    error
}

func (s *lazySchema) get(reason string) Schema {
	s.once.Do(func() {
		s.schema, s.err = s.load()
	})
	if s.err != nil {
		panic(fmt.Errorf("failed to load Terraform Plugin Framework %s schema %s while handling %s: %w",
			s.kind, s.tfName, reason, s.err))
	}
	return s.schema
}

func (s *lazySchema) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return s.get("ApplyTerraform5AttributePathStep").ApplyTerraform5AttributePathStep(step)
}

func (s *lazySchema) Type(ctx context.Context) tftypes.Type {
	return s.get("Type").Type(ctx)
}

func (s *lazySchema) ResourceSchemaVersion() int64 {
	return s.get("ResourceSchemaVersion").ResourceSchemaVersion()
}

func (s *lazySchema) Attrs() map[string]Attr {
	return s.get("Attrs").Attrs()
}

func (s *lazySchema) Blocks() map[string]Block {
	return s.get("Blocks").Blocks()
}

func (s *lazySchema) DeprecationMessage() string {
	return s.get("DeprecationMessage").DeprecationMessage()
}

func (s *lazySchema) ResourceProtoSchema(ctx context.Context) (*tfprotov6.Schema, error) {
	return s.get("ResourceProtoSchema").ResourceProtoSchema(ctx)
}

func (s *lazySchema) TFName() runtypes.TypeName {
	return s.tfName
}
