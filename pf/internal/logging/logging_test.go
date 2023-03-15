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

package logging

import (
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupRootLoggers(t *testing.T) {
	var buf bytes.Buffer
	ctx := SetupRootLoggers(context.Background(), &buf)
	tflog.Error(ctx, "Something went wrong")
	assert.Regexp(t, `\[ERROR\] logging/logging_test.go:\d+: provider: Something went wrong\s*$`, buf.String())
}

func TestParseLevelFromRawString(t *testing.T) {
	msg := "2023-03-15T10:52:48.612-0500 [ERROR] provider/resource_integer.go:113: provider: Create RandomInteger - ERROR +fields: superfield=supervalue a=1 b=b"
	require.Equal(t, hclog.Error, parseLevelFromRawString(msg))
}
