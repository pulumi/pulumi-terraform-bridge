// Copyright 2016-2022, Pulumi Corporation.
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
