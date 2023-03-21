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

package convert

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type EjectOptions struct {
	// AllowMissingProperties, if true, allows code-gen to continue even if the input configuration does not include.
	// values for required properties.
	AllowMissingProperties bool
	// AllowMissingProviders, if true, allows code-gen to continue even if resource providers are missing.
	AllowMissingProviders bool
	// AllowMissingVariables, if true, allows code-gen to continue even if the input configuration references missing
	// variables.
	AllowMissingVariables bool
	// AllowMissingComments allows binding to succeed even if there are errors extracting comments from the source.
	AllowMissingComments bool
	// AnnotateNodesWithLocations is true if the generated source code should contain comments that annotate top-level
	// nodes with their original source locations.
	AnnotateNodesWithLocations bool
	// FilterResourceNames, if true, removes the property indicated by ResourceNameProperty from all resources in the
	// graph.
	FilterResourceNames bool
	// ResourceNameProperty sets the key of the resource name property that will be removed if FilterResourceNames is
	// true.
	ResourceNameProperty string
	// Root, when set, overrides the default filesystem used to load the source Terraform module.
	Root afero.Fs
	// Optional package cache.
	PackageCache *pcl.PackageCache
	// Optional plugin host.
	PluginHost plugin.Host
	// Optional Loader.
	Loader schema.Loader
	// Optional source for provider schema information.
	ProviderInfoSource il.ProviderInfoSource
	// Optional logger for diagnostic information.
	Logger *log.Logger
	// SkipResourceTypechecking, if true, allows code-gen to continue even if resource inputs fail to typecheck.
	SkipResourceTypechecking bool
	// The target SDK version.
	TargetSDKVersion string
	// The version of Terraform targeteds by the input configuration.
	TerraformVersion string
}

// logf writes a formatted message to the configured logger, if any.
func (o EjectOptions) logf(format string, arguments ...interface{}) {
	if o.Logger != nil {
		o.Logger.Printf(format, arguments...)
	}
}

// Eject converts a Terraform module at the provided location into a Pulumi module.
func Eject(dir string, loader schema.ReferenceLoader, mapper convert.Mapper) (*workspace.Project, *pcl.Program, error) {
	return ejectWithOpts(dir, loader, mapper, nil)
}

// Used for testing so we can check eject with partial options set works.
func ejectWithOpts(dir string, loader schema.ReferenceLoader, mapper convert.Mapper,
	setOpts func(*EjectOptions)) (*workspace.Project, *pcl.Program, error) {

	if loader == nil {
		panic("must provide a non-nil loader")
	}

	opts := EjectOptions{
		Root:               afero.NewBasePathFs(afero.NewOsFs(), dir),
		Loader:             loader,
		ProviderInfoSource: il.NewMapperProviderInfoSource(mapper),
	}
	if setOpts != nil {
		setOpts(&opts)
	}

	tfFiles, program, diags, err := internalEject(opts)

	d := Diagnostics{All: diags, files: tfFiles}
	diagWriter := d.NewDiagnosticWriter(os.Stderr, 0, true)
	if len(diags) != 0 {
		err := diagWriter.WriteDiagnostics(diags)
		if err != nil {
			return nil, nil, err
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load Terraform configuration, %v", err)
	}
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("failed to load Terraform configuration, %v", diags)
	}

	project := &workspace.Project{
		Name: tokens.PackageName(filepath.Base(dir)),
	}

	return project, program, nil
}

func internalEject(opts EjectOptions) ([]*syntax.File, *pcl.Program, hcl.Diagnostics, error) {
	// Set default options where appropriate.
	if opts.ProviderInfoSource == nil {
		opts.ProviderInfoSource = il.PluginProviderInfoSource
	}

	// Attempt to load the config as TF11 first. If this succeeds, use TF11 semantics unless either the config
	// or the options specify otherwise.
	generatedFiles, tf11Err := convertTF11(opts)

	var tf12Files []*syntax.File
	var diagnostics hcl.Diagnostics

	if tf11Err == nil {
		// Parse the config.
		parser := syntax.NewParser()
		for filename, contents := range generatedFiles {
			err := parser.ParseFile(bytes.NewReader(contents), filename)
			contract.Assertf(err == nil, "err != nil")
		}
		tf12Files, diagnostics = parser.Files, append(diagnostics, parser.Diagnostics...)
		if diagnostics.HasErrors() {
			return tf12Files, nil, diagnostics, nil
		}
	} else {
		tf12Files, diagnostics = parseTF12(opts)
		if diagnostics.HasErrors() {
			return tf12Files, nil, diagnostics, nil
		}
	}

	tf12Files, program, programDiags, err := convertTF12(tf12Files, opts)
	if err != nil {
		return nil, nil, nil, err
	}

	diagnostics = append(diagnostics, programDiags...)
	if diagnostics.HasErrors() {
		return tf12Files, nil, diagnostics, nil
	}
	return tf12Files, program, diagnostics, nil
}
