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

package tfgen

import (
	"github.com/pkg/errors"
	nodegen "github.com/pulumi/pulumi/pkg/codegen/nodejs"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"
)

type nodejsGenerator struct {
	pkg         string
	version     string
	info        tfbridge.ProviderInfo
	overlaysDir string
	outDir      string
}

// newNodeJSGenerator returns a language generator that understands how to produce Go packages.
func newNodeJSGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &nodejsGenerator{
		pkg:         pkg,
		version:     version,
		info:        info,
		overlaysDir: overlaysDir,
		outDir:      outDir,
	}
}

// typeName returns a type name for a given resource type.
func (g *nodejsGenerator) typeName(r *resourceType) string {
	return r.name
}

// emitPackage emits an entire package pack into the configured output directory with the configured settings.
func (g *nodejsGenerator) emitPackage(pack *pkg) error {
	ppkg, err := genPulumiSchema(pack, g.pkg, g.version, g.info)
	if err != nil {
		return errors.Wrap(err, "generating Pulumi schema")
	}

	var extraTSFiles map[string][]byte
	if jsi := g.info.JavaScript; jsi != nil && jsi.Overlay != nil {
		extraTSFiles, err = getOverlayFiles(jsi.Overlay, ".ts", g.outDir)
		if err != nil {
			return err
		}
	}

	files, err := nodegen.GeneratePackage(tfgen, ppkg, extraTSFiles)
	if err != nil {
		return errors.Wrap(err, "generating Pulumi package")
	}

	for f, contents := range files {
		if err := emitFile(g.outDir, f, contents); err != nil {
			return errors.Wrapf(err, "emitting file %v", f)
		}
	}
	return nil
}
