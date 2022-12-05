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
)

func AssertEqualsJSONFile[T any](
	t *testing.T,
	expectedJSONFile string,
	actualData T,
) {
	var empty T

	unmarshalT := func(r io.Reader) (s T, err error) {
		err = json.NewDecoder(r).Decode(&s)
		return
	}

	readTFromFile := func(file string) (T, error) {
		jsonBytes, err := os.ReadFile(file)
		if err != nil {
			return empty, err
		}
		return unmarshalT(bytes.NewBuffer(jsonBytes))
	}

	marshalT := func(s T, w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	tToString := func(s T) string {
		buf := bytes.Buffer{}
		err := marshalT(s, &buf)
		assert.NoError(t, err)
		return buf.String()
	}

	assertDataMatchesFile := func(actualData T, file string) {
		expectedData, err := readTFromFile(file)
		assert.NoError(t, err)
		assert.Equal(t, tToString(expectedData), tToString(actualData))
	}

	if os.Getenv("PULUMI_ACCEPT") != "" {
		buf := bytes.Buffer{}
		err := marshalT(actualData, &buf)
		assert.NoError(t, err)
		err = os.WriteFile(expectedJSONFile, buf.Bytes(), 0600)
		assert.NoError(t, err)
	}

	assertDataMatchesFile(actualData, expectedJSONFile)
}
