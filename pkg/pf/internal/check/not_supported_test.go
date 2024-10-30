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

package check

import (
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/require"
)

func TestZeroRecognizer(t *testing.T) {
    t.Parallel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	u := &notSupportedUtil{
		sink: diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{Color: colors.Never}),
	}

	u.assertIsZero("key", "value")
	require.Contains(t, stderr.String(), "key received a non-zero custom value")
}
