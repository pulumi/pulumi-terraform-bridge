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

package parse

import (
	"bytes"
	"os"
	"runtime"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
)

func TestRenderTable(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows to avoid failing on incorrect newline handling")
	}

	tests := []struct {
		name     string
		input    string
		expected autogold.Value
	}{
		{
			name: "basic",
			input: `# hi

| t1 | t2 |
|---|---|
| r1c1 | r1c2 |
| r2c1 | r2c2 |
`,
			expected: autogold.Expect(`# hi

|  t1  |  t2  |
|------|------|
| r1c1 | r1c2 |
| r2c1 | r2c2 |
`),
		},
		{
			name: "with-in table effects",
			input: `
|  t1  |  *t2*  |
|------|------|
| __r1c1__ | r1c2 |
| r2c1 | r2c2 |
`,
			expected: autogold.Expect(`
|    t1    | *t2* |
|----------|------|
| **r1c1** | r1c2 |
| r2c1     | r2c2 |
`),
		},
		{
			name: "with-codeblock-after-text",
			input: "|  t1  | *t2*              |\n" +
				"|------|-------------------|\n" +
				"| r2c1 | r1c2 `code_block` |\n",
			expected: autogold.Expect("\n" +
				"|  t1  |       *t2*        |\n" +
				"|------|-------------------|\n" +
				"| r2c1 | r1c2 `code_block` |\n"),
		},
		{
			name:     "from-newrelic",
			input:    readfile(t, "../test_data/table-rendering/input.md"),
			expected: autogold.Expect(readfile(t, "../test_data/table-rendering/expected.md")),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := goldmark.New(
				goldmark.WithExtensions(TFRegistryExtension),
			).Convert([]byte(tt.input), &out)
			require.NoError(t, err)
			tt.expected.Equal(t, out.String())
		})
	}
}

func readfile(t *testing.T, file string) string {
	t.Helper()
	bytes, err := os.ReadFile(file)
	require.NoError(t, err)
	return string(bytes)
}
