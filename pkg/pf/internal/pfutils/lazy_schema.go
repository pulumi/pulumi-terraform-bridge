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
	ctx    context.Context
	load   func(context.Context) (Schema, error)

	mu     sync.Mutex
	loaded bool
	schema Schema
	err    error
}

func (s *lazySchema) get(ctx context.Context, reason string) Schema {
	s.mu.Lock()
	if !s.loaded {
		loadCtx := s.loadContext(ctx)
		schema, err := s.loadSchema(loadCtx)
		if err != nil && ctx != nil && loadCtx.Err() != nil {
			s.mu.Unlock()
			panic(s.loadError(reason, err))
		}
		s.schema, s.err = schema, err
		s.loaded = true
	}
	err := s.err
	schema := s.schema
	if schema == nil && err == nil {
		err = fmt.Errorf("schema loader returned nil schema without error")
		s.err = err
	}
	s.mu.Unlock()

	if err != nil {
		panic(s.loadError(reason, err))
	}
	return schema
}

func (s *lazySchema) loadContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return withoutCancel(s.ctx)
}

func (s *lazySchema) loadSchema(ctx context.Context) (schema Schema, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return s.load(ctx)
}

func (s *lazySchema) loadError(reason string, err error) error {
	return fmt.Errorf("failed to load Terraform Plugin Framework %s schema %s while handling %s: %w",
		s.kind, s.tfName, reason, err)
}

func (s *lazySchema) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return s.get(nil, "ApplyTerraform5AttributePathStep").ApplyTerraform5AttributePathStep(step)
}

func (s *lazySchema) Type(ctx context.Context) tftypes.Type {
	return s.get(ctx, "Type").Type(ctx)
}

func (s *lazySchema) ResourceSchemaVersion() int64 {
	return s.get(nil, "ResourceSchemaVersion").ResourceSchemaVersion()
}

func (s *lazySchema) Attrs() map[string]Attr {
	return s.get(nil, "Attrs").Attrs()
}

func (s *lazySchema) Blocks() map[string]Block {
	return s.get(nil, "Blocks").Blocks()
}

func (s *lazySchema) DeprecationMessage() string {
	return s.get(nil, "DeprecationMessage").DeprecationMessage()
}

func (s *lazySchema) ResourceProtoSchema(ctx context.Context) (*tfprotov6.Schema, error) {
	return s.get(ctx, "ResourceProtoSchema").ResourceProtoSchema(ctx)
}

func (s *lazySchema) TFName() runtypes.TypeName {
	return s.tfName
}
