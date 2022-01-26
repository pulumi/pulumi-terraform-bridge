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
	"fmt"

	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type languageBackend interface {
	name() string
	shouldConvertExamples() bool
	overlayInfo(info *tfbridge.ProviderInfo) *tfbridge.OverlayInfo
	emitFiles(spec *pschema.PackageSpec, overlay *tfbridge.OverlayInfo, root afero.Fs) (map[string][]byte, error)
}

func initializeLanguageBackend(lang Language) (languageBackend, error) {
	switch lang {
	case Golang:
		return &golangLanguageBackend{}, nil
	case NodeJS:
		return &nodejsLanguageBackend{}, nil
	case Python:
		return &pythonLanguageBackend{}, nil
	case CSharp:
		return &csharpLanguageBackend{}, nil
	case Schema:
		return &schemaLanguageBackend{}, nil
	}
	return nil, fmt.Errorf("%v does not support SDK generation", lang)
}
