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
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type pythonLanguageBackend struct{}

var _ languageBackend = &pythonLanguageBackend{}

func (b *pythonLanguageBackend) name() string {
	return string(Python)
}

func (b *pythonLanguageBackend) shouldConvertExamples() bool {
	return true
}

func (b *pythonLanguageBackend) overlayInfo(info *tfbridge.ProviderInfo) *tfbridge.OverlayInfo {
	if pyinfo := info.Python; pyinfo != nil {
		return pyinfo.Overlay
	}
	return nil
}

func (b *pythonLanguageBackend) emitFiles(
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

func (b *pythonLanguageBackend) emitSDK(
	pkg *pschema.Package,
	overlay *tfbridge.OverlayInfo,
	root afero.Fs,
) (map[string][]byte, error) {
	var extraFiles map[string][]byte
	var err error

	if overlay != nil {
		extraFiles, err = getOverlayFiles(overlay, ".py", root)
		if err != nil {
			return nil, err
		}
	}

	// python's outdir path follows the pattern [provider]/sdk/python/pulumi_[pkg name]
	pyOutDir := fmt.Sprintf("pulumi_%s", pkg.Name)
	err = cleanDir(root, pyOutDir, nil)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return pygen.GeneratePackage(tfgen, pkg, extraFiles)
}
