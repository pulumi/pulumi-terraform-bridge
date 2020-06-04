// Copyright 2016-2018, Pulumi Corporation.
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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
// nolint: goconst
package tfgen

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	nodejsgen "github.com/pulumi/pulumi/pkg/v2/codegen/nodejs"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
)

type nodeJSGenerator struct {
	pkg         string
	version     string
	info        tfbridge.ProviderInfo
	overlaysDir string
	outDir      string
}

// newNodeJSGenerator returns a language generator that understands how to produce Type/JavaScript packages.
func newNodeJSGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &nodeJSGenerator{
		pkg:         pkg,
		version:     version,
		info:        info,
		overlaysDir: overlaysDir,
		outDir:      outDir,
	}
}

// typeName returns a type name for a given resource type.
func (g *nodeJSGenerator) typeName(r *resourceType) string {
	return r.name
}

// emitPackage emits an entire package pack into the configured output directory with the configured settings.
func (g *nodeJSGenerator) emitPackage(pack *pkg) error {
	ppkg, err := genPulumiSchema(pack, g.pkg, g.version, g.info)
	if err != nil {
		return errors.Wrap(err, "generating Pulumi schema")
	}

	var extraNodeJSFiles map[string][]byte
	if psi := g.info.JavaScript; psi != nil && psi.Overlay != nil {
		extraNodeJSFiles, err = getOverlayFiles(psi.Overlay, ".ts", g.outDir)
		if err != nil {
			return err
		}
	}

	files, err := nodejsgen.GeneratePackage(tfgen, ppkg, extraNodeJSFiles)
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
