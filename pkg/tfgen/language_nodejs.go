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
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	nodejsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type nodejsLanguageBackend struct{}

var _ languageBackend = &nodejsLanguageBackend{}

func (b *nodejsLanguageBackend) name() string {
	return string(NodeJS)
}

func (b *nodejsLanguageBackend) shouldConvertExamples() bool {
	return true
}

func (b *nodejsLanguageBackend) overlayInfo(info *tfbridge.ProviderInfo) *tfbridge.OverlayInfo {
	if jsinfo := info.JavaScript; jsinfo != nil {
		return jsinfo.Overlay
	}
	return nil
}

func (b *nodejsLanguageBackend) emitFiles(
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

func (b *nodejsLanguageBackend) emitSDK(
	pkg *pschema.Package,
	overlay *tfbridge.OverlayInfo,
	root afero.Fs,
) (map[string][]byte, error) {
	var extraFiles map[string][]byte
	var err error

	if overlay != nil {
		extraFiles, err = getOverlayFiles(overlay, ".ts", root)
		if err != nil {
			return nil, err
		}
	}

	// We exclude the "tests" directory because some nodejs package dirs (e.g. pulumi-docker)
	// store tests here. We don't want to include them in the overlays because we don't want it
	// exported with the module, but we don't want them deleted in a cleanup of the directory.
	exclusions := codegen.NewStringSet("tests")

	// We don't need to add overlays to the exclusion list because they have already been read
	// into memory so deleting the files is not a problem.
	err = cleanDir(root, "", exclusions)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return nodejsgen.GeneratePackage(tfgen, pkg, extraFiles)
}
