package testing

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func AssertPackageSpecEquals(
	t *testing.T,
	expectedSchemaJSONFile string,
	spec pschema.PackageSpec,
) {

	marshalSchema := func(s pschema.PackageSpec, w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	assertPackageSpecMatchesFile := func(spec pschema.PackageSpec, file string) {
		expectedBytes, err := os.ReadFile(file)
		assert.NoError(t, err)
		expected := string(expectedBytes)

		buf := bytes.Buffer{}
		err = marshalSchema(spec, &buf)
		assert.NoError(t, err)
		actual := buf.String()

		assert.Equal(t, expected, actual)
	}

	if os.Getenv("PULUMI_ACCEPT") != "" {
		buf := bytes.Buffer{}
		err := marshalSchema(spec, &buf)
		assert.NoError(t, err)
		err = os.WriteFile(expectedSchemaJSONFile, buf.Bytes(), 0600)
		assert.NoError(t, err)
	}

	assertPackageSpecMatchesFile(spec, expectedSchemaJSONFile)
}
