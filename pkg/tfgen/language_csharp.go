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
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	dotnetgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type csharpLanguageBackend struct{}

var _ languageBackend = &csharpLanguageBackend{}

func (b *csharpLanguageBackend) name() string {
	return string(CSharp)
}

func (b *csharpLanguageBackend) shouldConvertExamples() bool {
	return true
}

func (b *csharpLanguageBackend) overlayInfo(info *tfbridge.ProviderInfo) *tfbridge.OverlayInfo {
	if cinfo := info.CSharp; cinfo != nil {
		return cinfo.Overlay
	}
	return nil
}

func (b *csharpLanguageBackend) emitFiles(
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

func (b *csharpLanguageBackend) emitSDK(
	pkg *pschema.Package,
	overlay *tfbridge.OverlayInfo,
	root afero.Fs,
) (map[string][]byte, error) {
	var extraFiles map[string][]byte
	var err error

	if overlay != nil {
		extraFiles, err = getOverlayFiles(overlay, ".cs", root)
		if err != nil {
			return nil, err
		}
	}
	err = cleanDir(root, "", nil)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return dotnetgen.GeneratePackage(tfgen, pkg, extraFiles)
}
