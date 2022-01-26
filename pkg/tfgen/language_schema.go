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

package tfgen

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type schemaLanguageBackend struct{}

var _ languageBackend = &schemaLanguageBackend{}

func (b *schemaLanguageBackend) name() string {
	return string(Schema)
}

func (b *schemaLanguageBackend) shouldConvertExamples() bool {
	return true
}

func (b *schemaLanguageBackend) overlayInfo(info *tfbridge.ProviderInfo) *tfbridge.OverlayInfo {
	return nil
}

func (b *schemaLanguageBackend) emitFiles(
	pulumiPackageSpec *pschema.PackageSpec,
	overlay *tfbridge.OverlayInfo,
	root afero.Fs,
) (map[string][]byte, error) {
	// Omit the version so that the spec is stable if the version is e.g. derived from the current Git commit hash.
	var pkgSpec pschema.PackageSpec = *pulumiPackageSpec
	pkgSpec.Version = ""
	bytes, err := json.MarshalIndent(pkgSpec, "", "    ")
	if err != nil {
		return map[string][]byte{}, errors.Wrapf(err, "failed to marshal schema")
	}
	return map[string][]byte{"schema.json": bytes}, nil
}
