package schemafilter

import (
	"bytes"
	"os"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

func TestFilterSchemaByLanguage(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                        string
		inputSchema                 []byte
		expectedLanguageSchemaBytes []byte
		language                    string
		// generator                   *Generator
	}{
		{
			name:        "Generates nodejs schema",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema.json")),
			language:    "nodejs",
		},
		{
			name:        "Generates python schema",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema.json")),
			language:    "python",
		},
		{
			name:        "Generates dotnet schema",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema.json")),
			language:    "dotnet",
		},
		{
			name:        "Generates go schema",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema.json")),
			language:    "go",
		},
		{
			name:        "Generates yaml schema",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema.json")),
			language:    "yaml",
		},
		{
			name:        "Generates java schema",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema.json")),
			language:    "java",
		},
		{
			name:        "Handles property names that are not surrounded by back ticks",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema-no-backticks.json")),
			language:    "nodejs",
		},
		{
			name:        "Handles property names that are surrounded by back ticks AND double quotes",
			inputSchema: []byte(readfile(t, "testdata/TestFilterSchemaByLanguage/schema-backticks-and-quotes.json")),
			language:    "nodejs",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := FilterSchemaByLanguage(tc.inputSchema, tc.language)
			hasSpan := bytes.Contains(actual, []byte("span"))
			require.False(t, hasSpan, "there should be no spans in the filtered schema")
			hasCodeChoosers := bytes.Contains(actual, []byte("PulumiCodeChooser"))
			require.False(t, hasCodeChoosers)

			autogold.ExpectFile(t, autogold.Raw(actual))
		})
	}
}

func readfile(t *testing.T, file string) string {
	t.Helper()
	bytes, err := os.ReadFile(file)
	require.NoError(t, err)
	return string(bytes)
}
