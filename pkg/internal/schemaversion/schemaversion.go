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

// Package schemaversion fills in the version of a Pulumi package schema at runtime.
package schemaversion

import (
	"bytes"
	"encoding/json"

	"github.com/blang/semver"
)

// Inject returns pulumiSchema with its top-level "version" set to version if the schema does not
// already declare one.
//
// tfgen omits the version from schema.json so that the file stays stable across releases. The
// provider binary knows its own version, so GetSchema fills it in. Consumers such as
// `pulumi package add` reject a schema without a version.
//
// pulumiSchema is returned unchanged if it already has a version, if version is not a valid semver
// string, or if pulumiSchema is not a JSON object.
func Inject(pulumiSchema []byte, version string) []byte {
	if len(pulumiSchema) == 0 || version == "" {
		return pulumiSchema
	}
	v, err := semver.ParseTolerant(version)
	if err != nil {
		return pulumiSchema
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(pulumiSchema, &fields); err != nil || fields == nil {
		return pulumiSchema
	}
	if existing, ok := fields["version"]; ok {
		var s string
		if err := json.Unmarshal(existing, &s); err != nil || s != "" {
			return pulumiSchema
		}
	}

	encodedVersion, err := json.Marshal(v.String())
	if err != nil {
		return pulumiSchema
	}
	fields["version"] = encodedVersion

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(fields); err != nil {
		return pulumiSchema
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}
