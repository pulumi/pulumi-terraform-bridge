// Copyright 2016-2020, Pulumi Corporation.
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
	pygen "github.com/pulumi/pulumi/pkg/v2/codegen/python"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
)

type pythonGenerator struct {
	pkg         string
	version     string
	info        tfbridge.ProviderInfo
	overlaysDir string
	outDir      string
}

// newPythonGenerator returns a language generator that understands how to produce Go packages.
func newPythonGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &pythonGenerator{
		pkg:         pkg,
		version:     version,
		info:        info,
		overlaysDir: overlaysDir,
		outDir:      outDir,
	}
}

// typeName returns a type name for a given resource type.
func (g *pythonGenerator) typeName(r *resourceType) string {
	return r.name
}

// emitPackage emits an entire package pack into the configured output directory with the configured settings.
func (g *pythonGenerator) emitPackage(pack *pkg) error {
	ppkg, err := genPulumiSchema(pack, g.pkg, g.version, g.info)
	if err != nil {
		return errors.Wrap(err, "generating Pulumi schema")
	}

	var extraPythonFiles map[string][]byte
	if psi := g.info.Python; psi != nil && psi.Overlay != nil {
		extraPythonFiles, err = getOverlayFiles(psi.Overlay, ".py", g.outDir)
		if err != nil {
			return err
		}
	}

	files, err := pygen.GeneratePackage(tfgen, ppkg, extraPythonFiles)
	if err != nil {
		return errors.Wrap(err, "generating Pulumi package")
	}

	for f, contents := range files {
		if f == "README.md" {
			// Do not overwrite the root-level README.md if it exists.
			if _, err := os.Stat(filepath.Join(g.outDir, f)); err == nil {
				continue
			}
		}
		if err := emitFile(g.outDir, f, contents); err != nil {
			return errors.Wrapf(err, "emitting file %v", f)
		}
	}
	return nil
}
