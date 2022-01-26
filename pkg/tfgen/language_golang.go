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
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type golangLanguageBackend struct{}

var _ languageBackend = &golangLanguageBackend{}

func (b *golangLanguageBackend) name() string {
	return string(Golang)
}

func (b *golangLanguageBackend) shouldConvertExamples() bool {
	return true
}

func (b *golangLanguageBackend) overlayInfo(info *tfbridge.ProviderInfo) *tfbridge.OverlayInfo {
	if goinfo := info.Golang; goinfo != nil {
		return goinfo.Overlay
	}
	return nil
}

func (b *golangLanguageBackend) emitFiles(
	pulumiPackageSpec *pschema.PackageSpec,
	overlay *tfbridge.OverlayInfo,
	root afero.Fs,
) (map[string][]byte, error) {
	empty := map[string][]byte{}
	pulumiPackage, err := pschema.ImportSpec(*pulumiPackageSpec, nil)
	if err != nil {
		return empty, errors.Wrapf(err, "failed to import Pulumi schema")
	}
	files, err := b.emitSDK(pulumiPackage, overlay, root)
	if err != nil {
		return empty, errors.Wrapf(err, "failed to generate package")
	}
	return files, nil
}

func (b *golangLanguageBackend) emitSDK(
	pkg *pschema.Package,
	overlay *tfbridge.OverlayInfo,
	root afero.Fs,
) (map[string][]byte, error) {
	return gogen.GeneratePackage(tfgen, pkg)
}
