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
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	hcl2java "github.com/pulumi/pulumi-java/pkg/codegen/java"
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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/autofill"
)

const (
	exampleUnavailable = "Example currently unavailable in this language\n"
)

func cliConverterEnabled() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_CONVERT"))
}

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
	info         tfbridge.ProviderInfo // provider declaration
	pluginHost   plugin.Host           // the plugin host for PCL conversion
	packageCache *pcl.PackageCache     // the package cache for PCL conversion

	hcls map[string]struct{} // set of observed HCL snippets

	generator interface {
		convertHCL(
			e *Example, hcl, path string, languages []string,
		) (string, error)
		convertExamplesInner(
			docs string,
			path examplePath,
			convertHCL func(
				e *Example, hcl, path string, languages []string,
			) (string, error),
			useCoverageTracker bool,
		) string
		getOrCreateExamplesCache() *examplesCache
	}

	loader pschema.Loader

	convertExamplesList []struct {
		docs string
		path examplePath
	}

	currentPackageSpec *pschema.PackageSpec

	pcls map[string]translatedExample // translations indexed by HCL
	opts []pcl.BindOption             // options cache; do not set
}

// Represents a partially converted example. PCL is the Pulumi dialect of HCL.
type translatedExample struct {
	PCL         string          `json:"pcl"`
	PulumiYAML  string          `json:"pulumiYaml"`
	Diagnostics hcl.Diagnostics `json:"diagnostics"`
}

// Get or create the cliConverter associated with the Generator.
func (g *Generator) cliConverter() *cliConverter {
	if g.cliConverterState != nil {
		return g.cliConverterState
	}
	g.cliConverterState = &cliConverter{
		generator:    g,
		hcls:         map[string]struct{}{},
		info:         g.info,
		packageCache: g.packageCache,
		pluginHost:   g.pluginHost,
		pcls:         map[string]translatedExample{},
	}
	if g.pluginHost != nil {
		l := newLoader(g.pluginHost)
		// Ensure azurerm resolves to azure for example:
		l.aliasPackage(g.info.Name, string(g.pkg))
		g.cliConverterState.loader = l
	}
	return g.cliConverterState
}

// Instead of converting examples, detect HCL literals involved and record placeholders for later.
func (cc *cliConverter) StartConvertingExamples(
	docs string,
	path examplePath,
) string {
	// Record inner HCL conversions and discard the result.
	cov := false // do not use coverage tracker yet, it will be used in the second pass.
	cc.generator.convertExamplesInner(docs, path, cc.recordHCL, cov)
	// Record the convertExamples job for later.
	e := struct {
		docs string
		path examplePath
	}{
		docs: docs,
		path: path,
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

		// Use coverage tracker here on the second pass.
		useCoverageTracker := true
		source := cc.generator.convertExamplesInner(ex.docs, ex.path, cc.generator.convertHCL, useCoverageTracker)
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

const cliConverterErrUnexpectedHCLSnippet = "unexpected HCL snippet in Convert"

// During FinishConvertingExamples pass, generator calls back into this function to continue
// PCL->lang translation from a pre-computed HCL->PCL translation table cc.pcls.
func (cc *cliConverter) Convert(
	hclCode string, lang string,
) (converted string, diags hcl.Diagnostics, ferr error) {
	example, ok := cc.pcls[hclCode]
	// Cannot assert here because panics are still possible for some reason.
	// Example: gcp:gameservices/gameServerCluster:GameServerCluster
	//
	// Something skips adding failing conversion diagnostics to cc.pcls when pre-converting. The
	// end-user experience is not affected much, the above example does not regress.
	if !ok {
		return "", hcl.Diagnostics{}, fmt.Errorf("%s %q", cliConverterErrUnexpectedHCLSnippet, hclCode)
	}
	if example.Diagnostics.HasErrors() {
		return "", example.Diagnostics, nil
	}
	source, diags, err := cc.convertPCL(example.PCL, lang)
	return source, cc.postProcessDiagnostics(diags.Extend(example.Diagnostics)), err
}

// Convert all observed HCL snippets from cc.hcls to PCL in one pass, populate cc.pcls.
func (cc *cliConverter) bulkConvert() error {
	if len(cc.hcls) == 0 {
		return nil
	}
	examples := map[string]string{}
	n := 0
	for hcl := range cc.hcls {
		fileName := fmt.Sprintf("e%d", n)
		examples[fileName] = hcl
		n++
	}
	result, err := cc.convertViaPulumiCLI(cc.autoFill(examples), []tfbridge.ProviderInfo{
		cc.info,
	})
	if err != nil {
		return err
	}
	for fileName, hcl := range examples {
		r := result[fileName]
		cc.pcls[hcl] = translatedExample{
			PCL:         r.PCL,
			Diagnostics: cc.postProcessDiagnostics(r.Diagnostics),
		}
	}
	return nil
}

func (cc *cliConverter) autoFill(examples map[string]string) map[string]string {
	if a, ok := autofill.ConfigureAutoFill(); ok {
		out := map[string]string{}
		for fileName, hcl := range examples {
			hclPlus, err := a.FillUndeclaredReferences(hcl)
			if err != nil {
				contract.IgnoreError(err)
				out[fileName] = hcl
			} else {
				out[fileName] = hclPlus
			}
		}
		return out
	}
	return examples
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
//
// The mappings argument helps the converter resolve the metadata for bridged providers during
// example translation. Most importantly it needs to include the current provider, but it also may
// include additional providers used in examples.
func (cc *cliConverter) convertViaPulumiCLI(
	examples map[string]string,
	mappings []tfbridge.ProviderInfo,
) (map[string]translatedExample, error) {
	translated, err := cc.convertViaPulumiCLIStep(examples, mappings)
	if err == nil {
		return translated, nil
	}

	// Try to bisect examples to a subset that still errors out.
	for len(examples) >= 2 {
		e1, e2 := cc.split2(examples)
		if _, err := cc.convertViaPulumiCLIStep(e1, mappings); err != nil {
			examples = e1
		} else if _, err := cc.convertViaPulumiCLIStep(e2, mappings); err != nil {
			examples = e2
		} else {
			break
		}
	}

	dir, err2 := cc.convertViaPulumiPrepareDebugFolder(examples, mappings)
	contract.IgnoreError(err2)

	return nil, fmt.Errorf("\n######\n  pulumi convert failed\n  minimal repro: %s\n  full error below\n######\n%w",
		dir, err)
}

func (*cliConverter) split2(xs map[string]string) (map[string]string, map[string]string) {
	h1 := map[string]string{}
	h2 := map[string]string{}
	keys := []string{}
	for k := range xs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys[0 : len(xs)/2] {
		h1[k] = xs[k]
	}
	for _, k := range keys[len(xs)/2:] {
		h2[k] = xs[k]
	}
	return h1, h2
}

// To help with debugging failures prepares a temp folder with a repro script and returns a path to it.
func (cc *cliConverter) convertViaPulumiPrepareDebugFolder(
	examples map[string]string,
	mappings []tfbridge.ProviderInfo,
) (string, error) {
	d, err := os.MkdirTemp("", "convert-examples-repro")
	if err != nil {
		return "", err
	}

	_, args, err := cc.convertViaPulumiCLICommandArgs(examples, mappings, d, filepath.Join(d, "examples.json"))
	if err != nil {
		return "", err
	}

	// write out all examples into tf files for easy consumption
	for k, v := range examples {
		err := os.WriteFile(filepath.Join(d, fmt.Sprintf("%s.tf", k)), []byte(v), 0o600)
		if err != nil {
			return "", err
		}
	}

	// write a repro.sh script to show how pulumi is invoked
	script := []byte(fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

pulumi %s
`, strings.Join(args, " ")))

	if err := os.WriteFile(filepath.Join(d, "repro.sh"), script, 0o600); err != nil {
		return "", err
	}

	return d, nil
}

func (cc *cliConverter) convertViaPulumiCLICommandArgs(
	examples map[string]string,
	mappings []tfbridge.ProviderInfo,
	outDir string,
	examplesJSONPath string,
) (string, []string, error) {
	// Write example to bridge-examples.json.
	examplesBytes, err := json.Marshal(examples)
	if err != nil {
		return "", nil, fmt.Errorf("convertViaPulumiCLI: failed to marshal examples to JSON: %w", err)
	}
	if err := os.WriteFile(examplesJSONPath, examplesBytes, 0o600); err != nil {
		return "", nil, fmt.Errorf("convertViaPulumiCLI: failed to write a temp examples.json file: %w", err)
	}

	pulumiPath, err := exec.LookPath("pulumi")
	if err != nil {
		return "", nil, fmt.Errorf("convertViaPulumiCLI: pulumi executable not in PATH: %w", err)
	}

	mappingsDir := filepath.Join(outDir, "mappings")

	// Prepare mappings folder if necessary.
	if len(mappings) > 0 {
		if err := os.MkdirAll(mappingsDir, 0o755); err != nil {
			return "", nil, fmt.Errorf("convertViaPulumiCLI: failed to write mappings folder: %w", err)
		}
	}

	// Write out mappings files if necessary.
	for _, info := range mappings {
		info := info // remove aliasing lint
		mpi := tfbridge.MarshalProviderInfo(&info)
		bytes, err := json.Marshal(mpi)
		if err != nil {
			return "", nil, fmt.Errorf("convertViaPulumiCLI: failed to write mappings folder: %w", err)
		}
		mf := cc.mappingsFile(mappingsDir, info)
		if err := os.WriteFile(mf, bytes, 0o600); err != nil {
			return "", nil, fmt.Errorf("convertViaPulumiCLI: failed to write mappings file: %w", err)
		}
	}

	var mappingsArgs []string
	for _, info := range mappings {
		mappingsArgs = append(mappingsArgs, "--mappings", cc.mappingsFile(mappingsDir, info))
	}

	cmdArgs := []string{
		"convert",
		"--from", "terraform",
		"--language", "pcl",
		"--out", outDir,
		"--generate-only",
	}

	cmdArgs = append(cmdArgs, mappingsArgs...)
	cmdArgs = append(cmdArgs, "--", "--convert-examples", filepath.Base(examplesJSONPath))
	return pulumiPath, cmdArgs, nil
}

func (cc *cliConverter) convertViaPulumiCLIStep(
	examples map[string]string,
	mappings []tfbridge.ProviderInfo,
) (
	output map[string]translatedExample,
	finalError error,
) {
	outDir, err := os.MkdirTemp("", "bridge-examples-output")
	if err != nil {
		return nil, fmt.Errorf("convertViaPulumiCLI: failed to create a temp dir "+
			" bridge-examples-output: %w", err)
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
		return nil, fmt.Errorf("convertViaPulumiCLI: failed to create a temp "+
			" bridge-examples.json file: %w", err)
	}
	defer func() {
		if err := os.Remove(examplesJSON.Name()); err != nil {
			if finalError == nil {
				finalError = fmt.Errorf("convertViaPulumiCLI: failed to clean up "+
					"temp bridge-examples.json file: %w", err)
			}
		}
	}()

	pulumiPath, cmdArgs, err := cc.convertViaPulumiCLICommandArgs(examples, mappings, outDir, examplesJSON.Name())
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(pulumiPath, cmdArgs...)

	cmd.Dir = filepath.Dir(examplesJSON.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("convertViaPulumiCLI: pulumi command failed: %w\n"+
			"Stdout:\n%s\n\n"+
			"Stderr:\n%s",
			err, stdout.String(), stderr.String())
	}

	outputFile := filepath.Join(outDir, filepath.Base(examplesJSON.Name()))

	outputFileBytes, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("convertViaPulumiCLI: failed to read output file: %w, "+
			"check if your Pulumi CLI version is recent enough to include pulumi-converter-terraform v1.0.9",
			err)
	}

	var result map[string]translatedExample
	if err := json.Unmarshal(outputFileBytes, &result); err != nil {
		return nil, fmt.Errorf("convertViaPulumiCLI: failed to unmarshal output "+
			"file: %w", err)
	}

	return result, nil
}

func (*cliConverter) mappingsFile(mappingsDir string, info tfbridge.ProviderInfo) string {
	// This seems to be what the converter expects the filename to be. For providers
	// like "aws" this is simply the provider name, but there are exceptions such as
	// "azure" where this has to be "azurerm.json" to match the prefix on the Terraform
	// resource names such as azurerm_xyz.
	name := info.GetResourcePrefix()
	return filepath.Join(mappingsDir, fmt.Sprintf("%s.json", name))
}

// Conversion from PCL to the target language still happens in-process temporarily, which is really
// unfortunate because it makes another plugin loader necessary. This should eventually also happen
// through pulumi convert, but it needs to have bulk interface enabled for every language.
func (cc *cliConverter) convertPCL(
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

	if cc.opts == nil {
		var opts []pcl.BindOption
		opts = append(opts, pcl.AllowMissingProperties)
		opts = append(opts, pcl.AllowMissingVariables)
		opts = append(opts, pcl.SkipResourceTypechecking)
		if cc.pluginHost != nil {
			opts = append(opts, pcl.PluginHost(cc.pluginHost))
		}
		if cc.loader != nil {
			opts = append(opts, pcl.Loader(cc.loader))
		}
		if cc.packageCache != nil {
			opts = append(opts, pcl.Cache(cc.packageCache))
		}
		cc.opts = opts
	}

	program, programDiags, err := pcl.BindProgram(pulumiParser.Files, cc.opts...)
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
		err = fmt.Errorf("unsupported language: %q", languageName)
	}
	if err != nil {
		return "", diagnostics, fmt.Errorf("generate program failed: %w", err)
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
	e *Example, hcl, path string, languages []string,
) (string, error) {
	cache := cc.generator.getOrCreateExamplesCache()

	allLanguagesCached := true
	for _, lang := range languages {
		if _, ok := cache.Lookup(hcl, lang); !ok {
			allLanguagesCached = false
		}
	}

	// Schedule bulk conversion of this HCL snippet unless it is certain that every language
	// translation has a cache hit, in which case the hcl->pcl translation will never be needed.
	if !allLanguagesCached {
		cc.hcls[hcl] = struct{}{}
	}

	return "{convertHCL}", nil
}

func (cc *cliConverter) postProcessDiagnostics(diag hcl.Diagnostics) hcl.Diagnostics {
	var out []*hcl.Diagnostic
	for _, d := range diag {
		copy := *d
		cc.removeFileName(&copy)
		cc.ensureNotYetImplementedIsAnError(&copy)
		cc.ensureNotSupportedLifecycleHooksIsError(&copy)
		out = append(out, &copy)
	}
	return out
}

func (*cliConverter) removeFileName(d *hcl.Diagnostic) {
	if d == nil {
		return
	}
	if d.Subject != nil {
		d.Subject.Filename = ""
	}
	if d.Context != nil {
		d.Context.Filename = ""
	}
}

var (
	notYetImplementedPattern         = regexp.MustCompile("(?i)not yet implemented")
	notSupportedLifecycleHookPattern = regexp.MustCompile("(?i)lifecycle hook is not supported")
)

func (*cliConverter) ensureNotYetImplementedIsAnError(d *hcl.Diagnostic) {
	if notYetImplementedPattern.MatchString(d.Error()) {
		d.Severity = hcl.DiagError
	}
}

func (*cliConverter) ensureNotSupportedLifecycleHooksIsError(d *hcl.Diagnostic) {
	if notSupportedLifecycleHookPattern.MatchString(d.Error()) {
		d.Severity = hcl.DiagError
	}
}

// Function for one-off example converson HCL --> PCL using pulumi-converter-terraform
func (cc *cliConverter) singleExampleFromHCLToPCL(path, hclCode string) (translatedExample, error) {
	key := path
	result, err := cc.convertViaPulumiCLI(map[string]string{key: hclCode}, []tfbridge.ProviderInfo{cc.info})
	if err != nil {
		return translatedExample{}, nil
	}
	return result[key], nil
}

// Function for one-off example conversions PCL --> supported language (nodejs, yaml, etc)
func (cc *cliConverter) singleExampleFromPCLToLanguage(example translatedExample, lang string) (string, error) {
	var err error

	source, diags, _ := cc.convertPCL(example.PCL, lang)
	diags = cc.postProcessDiagnostics(diags.Extend(example.Diagnostics))
	if diags.HasErrors() {
		err = fmt.Errorf("conversion errors: %s", diags.Error())
	}

	if source == "" {
		source = exampleUnavailable
	}
	source = "```" + lang + "\n" + source + "```"
	return source, err
}
