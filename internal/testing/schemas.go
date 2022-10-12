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

	unmarshalSchema := func(r io.Reader) (s pschema.PackageSpec, err error) {
		err = json.NewDecoder(r).Decode(&s)
		return
	}

	readSchemaFromFile := func(file string) (pschema.PackageSpec, error) {
		jsonBytes, err := os.ReadFile(file)
		if err != nil {
			return pschema.PackageSpec{}, err
		}
		return unmarshalSchema(bytes.NewBuffer(jsonBytes))
	}

	marshalSchema := func(s pschema.PackageSpec, w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	schemaToString := func(s pschema.PackageSpec) string {
		buf := bytes.Buffer{}
		err := marshalSchema(s, &buf)
		assert.NoError(t, err)
		return buf.String()
	}

	assertPackageSpecMatchesFile := func(actualSpec pschema.PackageSpec, file string) {
		expectedSpec, err := readSchemaFromFile(file)
		assert.NoError(t, err)
		assert.Equal(t, schemaToString(expectedSpec), schemaToString(actualSpec))
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
