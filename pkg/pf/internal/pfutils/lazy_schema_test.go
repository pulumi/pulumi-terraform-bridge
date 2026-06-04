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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
)

func TestLazySchemaPanicsWhenLoaderReturnsNilSchemaWithoutError(t *testing.T) {
	t.Parallel()

	var calls int
	schema := &lazySchema{
		kind:   "resource",
		tfName: runtypes.TypeName("test_thing"),
		ctx:    context.Background(),
		load: func(context.Context) (Schema, error) {
			calls++
			return nil, nil
		},
	}

	first := panicMessage(func() { schema.Type(context.Background()) })
	second := panicMessage(func() { schema.Type(context.Background()) })

	require.Contains(t, first, "failed to load Terraform Plugin Framework resource schema test_thing")
	require.Contains(t, first, "schema loader returned nil schema without error")
	require.Equal(t, first, second)
	require.Equal(t, 1, calls)
}

func panicMessage(f func()) (message string) {
	defer func() {
		if p := recover(); p != nil {
			message = fmt.Sprint(p)
		}
	}()
	f()
	return ""
}
