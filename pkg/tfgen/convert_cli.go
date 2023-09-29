// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/hashicorp/hcl/v2"

	hcl2java "github.com/pulumi/pulumi-java/pkg/codegen/java"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	hcl2yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	hcl2dotnet "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	hcl2go "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	hcl2nodejs "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	hcl2python "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var (
	cliConverterEnabled bool = cmdutil.IsTruthy(os.Getenv("PULUMI_CONVERT"))
)

// Integrates with `pulumi convert` command for converting TF examples.
//
// Pulumi CLI now supprts a handy `pulumi convert` command. This file implements integrating with
// this command for the purposes of initial conversion of Terraform examples into PCL language. This
// integration is preferable to linking the functionality in as it allows bridged providers to not
// build-depend on the TF converter.
//
// Note that once examples are converted to PCL, they continue to be processed with in-process
// target language specific generators to produce TypeScript, YAML, Python etc target code.
type cliConverter struct {
	packageName  string                // name of the provider such as "gcp"
	info         tfbridge.ProviderInfo // provider declaration
	pluginHost   plugin.Host           // the plugin host for PCL conversion
	packageCache *pcl.PackageCache     // the package cache for PCL conversion

	hcls map[string]struct{} // set of observed HCL snippets

	generator interface {
		convertHCL(hcl, path, exampleTitle string, languages []string) (string, error)
		convertExamplesInner(
			docs string,
			path examplePath,
			stripSubsectionsWithErrors bool,
			convertHCL func(hcl, path, exampleTitle string,
				languages []string) (string, error),
		) string
	}

	convertExamplesList []struct {
		docs                       string
		path                       examplePath
		stripSubsectionsWithErrors bool
	}

	currentPackageSpec *pschema.PackageSpec

	pcls map[string]translatedExample // translations indexed by HCL
	opts []pcl.BindOption             // options cache; do not set
}

// Represents a partially converted example. PCL is the Pulumi dialect of HCL.
type translatedExample struct {
	PCL         string          `json:"pcl"`
	Diagnostics hcl.Diagnostics `json:"diagnostics"`
}

// Get or create the cliConverter associated with the Generator.
func (g *Generator) cliConverter() *cliConverter {
	if g.cliConverterState != nil {
		return g.cliConverterState
	}
	packageName := string(g.pkg)
	infoCopy := g.info
	infoCopy.Name = packageName // correct "google-beta" to "gcp"
	g.cliConverterState = &cliConverter{
		generator:    g,
		hcls:         map[string]struct{}{},
		info:         infoCopy,
		packageCache: g.packageCache,
		packageName:  packageName,
		pluginHost:   g.pluginHost,
		pcls:         map[string]translatedExample{},
	}
	return g.cliConverterState
}

// Instead of converting examples, detect HCL literals involved and record placeholders for later.
func (cc *cliConverter) StartConvertingExamples(
	docs string,
	path examplePath,
	stripSubsectionsWithErrors bool,
) string {
	// Record inner HCL conversions and discard the result.
	cc.generator.convertExamplesInner(docs, path, stripSubsectionsWithErrors, cc.recordHCL)
	// Record the convertExamples job for later.
	e := struct {
		docs                       string
		path                       examplePath
		stripSubsectionsWithErrors bool
	}{
		docs:                       docs,
		path:                       path,
		stripSubsectionsWithErrors: stripSubsectionsWithErrors,
	}
	cc.convertExamplesList = append(cc.convertExamplesList, e)
	// Return a placeholder referencing the convertExampleJob by position.
	return fmt.Sprintf("{convertExamples:%d}", len(cc.convertExamplesList)-1)
}

// Replace all convertExamples placeholders with actual values by rendering them.
func (cc *cliConverter) FinishConvertingExamples(p pschema.PackageSpec) pschema.PackageSpec {
	// Remember partially constructed PackageSpec so that Convert can access it.
	cc.currentPackageSpec = &p

	err := cc.bulkConvert()
	contract.AssertNoErrorf(err, "bulk converting examples failed")

	bytes, err := json.Marshal(p)
	contract.AssertNoErrorf(err, "json.Marshal failed on PackageSpec")
	re := regexp.MustCompile("[{]convertExamples[:]([^}]+)[}]")

	// Convert all stubs populated by StartConvertingExamples.
	fixedBytes := re.ReplaceAllFunc(bytes, func(match []byte) []byte {
		groups := re.FindSubmatch(match)
		i, err := strconv.Atoi(string(groups[1]))
		contract.AssertNoErrorf(err, "strconv.Atoi")
		ex := cc.convertExamplesList[i]
		source := cc.generator.convertExamplesInner(ex.docs, ex.path,
			ex.stripSubsectionsWithErrors, cc.generator.convertHCL)
		// JSON-escaping to splice into JSON string literals.
		bytes, err := json.Marshal(source)
		contract.AssertNoErrorf(err, "json.Masrhal(sourceCode)")
		return bytes[1 : len(bytes)-1]
	})

	var result pschema.PackageSpec
	err = json.Unmarshal(fixedBytes, &result)
	contract.AssertNoErrorf(err, "json.Unmarshal failed to recover PackageSpec")
	return result
}

// During FinishConvertingExamples pass, generator calls back into this function to continue
// PCL->lang translation from a pre-computed HCL->PCL translation table cc.pcls.
func (cc *cliConverter) Convert(hclCode string, lang string) (string, hcl.Diagnostics, error) {
	example, ok := cc.pcls[hclCode]
	contract.Assertf(ok, "unexpected new HCL snippet in Convert; should have seen it before")
	if example.Diagnostics.HasErrors() {
		return "", example.Diagnostics, nil
	}
	source, diags, err := cc.convertPCL(cc.currentPackageSpec, example.PCL, lang)
	return source, diags.Extend(example.Diagnostics), err
}

// Convert all observed HCL snippets from cc.hcls to PCL in one pass, populate cc.pcls.
func (cc *cliConverter) bulkConvert() error {
	examples := map[string]string{}
	n := 0
	for hcl := range cc.hcls {
		examples[fmt.Sprintf("e%d", n)] = hcl
		n++
	}
	result, err := cc.convertViaPulumiCLI(examples, []tfbridge.ProviderInfo{cc.info})
	if err != nil {
		return err
	}
	for k, hcl := range examples {
		cc.pcls[hcl] = result[k]
	}
	return nil
}

// Calls pulumi convert to bulk-convert examples.
//
// To facilitate high-throughput conversion, an `examples.json` protocol is employed to convert
// examples in batches. See pulumi/pulumi-converter-terraform#29 for where the support is
// introduced.
//
// Source examples are passed in as a map from ID to raw TF code.
//
// This may need to be coarse-grain parallelized to speed up larger providers at the cost of more
// memory, for example run 4 instances of `pulumi convert` on 25% of examples each.
func (*cliConverter) convertViaPulumiCLI(
	examples map[string]string,
	mappings []tfbridge.ProviderInfo,
) (
	output map[string]translatedExample,
	finalError error,
) {
	outDir, err := os.MkdirTemp("", "bridge-examples-output")
	if err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: failed to create a temp dir "+
			" bridge-examples-output: %w", err)
		return
	}
	defer func() {
		if err := os.RemoveAll(outDir); err != nil {
			if finalError == nil {
				finalError = fmt.Errorf("convertViaPulumiCLI: failed to clean up "+
					"temp bridge-examples-output dir: %w", err)
			}
		}
	}()

	examplesJSON, err := os.CreateTemp("", "bridge-examples.json")
	if err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: failed to create a temp "+
			" bridge-examples.json file: %w", err)
		return
	}
	defer func() {
		if err := os.Remove(examplesJSON.Name()); err != nil {
			if finalError == nil {
				finalError = fmt.Errorf("convertViaPulumiCLI: failed to clean up "+
					"temp bridge-examples.json file: %w", err)
			}
		}
	}()

	// Write example to bridge-examples.json.
	examplesBytes, err := json.Marshal(examples)
	if err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: failed to marshal examples "+
			"to JSON: %w", err)
		return
	}
	if err := os.WriteFile(examplesJSON.Name(), examplesBytes, 0655); err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: failed to write a temp "+
			"bridge-examples.json file: %w", err)
		return
	}

	mappingsDir := filepath.Join(outDir, "mappings")

	mappingsFile := func(i int, p tfbridge.ProviderInfo) string {
		return filepath.Join(mappingsDir, fmt.Sprintf("%s.json", p.Name))
	}

	// Write out mappings files if necessary.
	for i, m := range mappings {
		if i == 0 {
			if err := os.MkdirAll(mappingsDir, 0755); err != nil {
				finalError = fmt.Errorf("convertViaPulumiCLI: failed to write "+
					"mappings folder: %w", err)
				return
			}
		}
		mpi := tfbridge.MarshalProviderInfo(&m)
		bytes, err := json.Marshal(mpi)
		if err != nil {
			finalError = fmt.Errorf("convertViaPulumiCLI: failed to write "+
				"mappings folder: %w", err)
			return
		}
		mf := mappingsFile(i, m)
		if err := os.WriteFile(mf, bytes, 0655); err != nil {
			finalError = fmt.Errorf("convertViaPulumiCLI: failed to write "+
				"mappings file: %w", err)
			return
		}
	}

	pulumiPath, err := exec.LookPath("pulumi")
	if err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: pulumi executalbe not "+
			"in PATH: %w", err)
		return
	}

	var mappingsArgs []string
	for i, m := range mappings {
		mappingsArgs = append(mappingsArgs, "--mappings", mappingsFile(i, m))
	}

	cmd := exec.Command(pulumiPath, append([]string{"convert",
		"--from", "terraform",
		"--language", "pcl",
		"--out", outDir,
		"--generate-only",
	}, mappingsArgs...)...)

	cmd.Dir = filepath.Dir(examplesJSON.Name())
	cmd.Env = append(os.Environ(),
		"PULUMI_CONVERT_EXAMPLES="+filepath.Base(examplesJSON.Name()))

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: pulumi command failed: %w\n"+
			"Stdout:\n%s\n\n"+
			"Stderr:\n%s\n\n",
			err, stdout.String(), stderr.String())
		return
	}

	outputFile := filepath.Join(outDir, filepath.Base(examplesJSON.Name()))

	outputFileBytes, err := os.ReadFile(outputFile)
	if err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: failed to read output file: %w", err)
		return
	}

	var result map[string]translatedExample
	if err := json.Unmarshal(outputFileBytes, &result); err != nil {
		finalError = fmt.Errorf("convertViaPulumiCLI: failed to unmarshal output "+
			"file: %w", err)
		return
	}

	return result, nil
}

// Conversion from PCL to the target language still happens in-process temporarily, which is really
// unfortunate because it makes another plugin loader necessary. This should eventually also happen
// through pulumi convert, but it needs to have bulk interface enabled for every language.
func (cc *cliConverter) convertPCL(
	spec *pschema.PackageSpec,
	source string,
	languageName string,
) (string, hcl.Diagnostics, error) {
	pulumiParser := syntax.NewParser()

	err := pulumiParser.ParseFile(bytes.NewBufferString(source), "example.pp")
	contract.AssertNoErrorf(err, "pulumiParser.ParseFile returned an error")

	var diagnostics hcl.Diagnostics

	diagnostics = append(diagnostics, pulumiParser.Diagnostics...)
	if diagnostics.HasErrors() {
		return "", diagnostics, nil
	}

	opts := cc.opts
	if opts == nil {
		var opts []pcl.BindOption
		opts = append(opts, pcl.AllowMissingProperties)
		opts = append(opts, pcl.AllowMissingVariables)
		opts = append(opts, pcl.SkipResourceTypechecking)
		if cc.pluginHost != nil {
			opts = append(opts, pcl.PluginHost(cc.pluginHost))
			loader := newLoader(cc.pluginHost)
			//loader.BindCurrentPackage(*spec, cc.packageName)
			opts = append(opts, pcl.Loader(loader))
		}
		if cc.packageCache != nil {
			opts = append(opts, pcl.Cache(cc.packageCache))
		}
		cc.opts = opts
	}

	program, programDiags, err := pcl.BindProgram(pulumiParser.Files, opts...)
	if err != nil {
		return "", diagnostics, fmt.Errorf("pcl.BindProgram failed: %w", err)
	}

	diagnostics = append(diagnostics, programDiags...)
	if diagnostics.HasErrors() {
		return "", diagnostics, nil
	}

	var genDiags hcl.Diagnostics
	var generatedFiles map[string][]byte

	switch languageName {
	case "typescript":
		generatedFiles, genDiags, err = hcl2nodejs.GenerateProgram(program)
		diagnostics = append(diagnostics, genDiags...)
	case "python":
		generatedFiles, genDiags, err = hcl2python.GenerateProgram(program)
		diagnostics = append(diagnostics, genDiags...)
	case "csharp":
		generatedFiles, genDiags, err = hcl2dotnet.GenerateProgram(program)
		diagnostics = append(diagnostics, genDiags...)
	case "go":
		generatedFiles, genDiags, err = hcl2go.GenerateProgram(program)
		diagnostics = append(diagnostics, genDiags...)
	case "yaml":
		generatedFiles, genDiags, err = hcl2yaml.GenerateProgram(program)
		diagnostics = append(diagnostics, genDiags...)
	case "java":
		generatedFiles, genDiags, err = hcl2java.GenerateProgram(program)
		diagnostics = append(diagnostics, genDiags...)
	default:
		err = fmt.Errorf("Unsupported language: %q", languageName)
	}
	if err != nil {
		return "", diagnostics, fmt.Errorf("GenerateProgram failed: %w", err)
	}
	if len(generatedFiles) != 1 {
		err := fmt.Errorf("expected 1 file to be generated, got %d", len(generatedFiles))
		return "", diagnostics, err
	}
	var key string
	for n := range generatedFiles {
		key = n
	}
	return string(generatedFiles[key]), diagnostics, nil
}

// Act as a convertHCL stub that does not actually convert but spies on the literals involved.
func (cc *cliConverter) recordHCL(
	hcl, path, exampleTitle string, languages []string,
) (string, error) {
	h := cc.hcls
	h[hcl] = struct{}{}
	return "{convertHCL}", nil
}
