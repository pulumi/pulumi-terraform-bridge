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

package unrec

import (
	"os"
	"path/filepath"
	"testing"

	"encoding/json"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Consults a generated test schema (see gen.go), run go generate to rebuild. This indirection allows the package not to
// build-depend on the tfgen module in case it ever needs to be exported from tfgen to avoid build cycles.
//
//go:generate go run gen.go
func exampleSchema(t *testing.T) *pschema.PackageSpec {
	b, err := os.ReadFile(filepath.Join("testdata", "test-schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r pschema.PackageSpec
	err = json.Unmarshal(b, &r)
	if err != nil {
		t.Fatal(err)
	}
	return &r
}
