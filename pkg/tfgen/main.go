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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"

	"github.com/golang/glog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Main executes the TFGen process for the given package pkg and provider prov.
func Main(pkg string, version string, prov tfbridge.ProviderInfo) {
	// Enable additional provider validation.
	schema.RunProviderInternalValidation = true

	MainWithCustomGenerate(pkg, version, prov, func(opts GeneratorOptions) error {
		// Create a generator with the specified settings.
		g, err := NewGenerator(opts)
		if err != nil {
			return err
		}

		// Let's generate some code!
		err = g.Generate()
		if err != nil {
			return err
		}

		return err
	})
}

// Like Main but allows to customize the generation logic past the parsing of cmd-line arguments.
func MainWithCustomGenerate(pkg string, version string, prov tfbridge.ProviderInfo,
	gen func(GeneratorOptions) error) {

	if err := newTFGenCmd(pkg, version, prov, gen).Execute(); err != nil {
		_, fmterr := fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		contract.IgnoreError(fmterr)
		os.Exit(-1)
	}
}

func newTFGenCmd(pkg string, version string, prov tfbridge.ProviderInfo,
	gen func(GeneratorOptions) error) *cobra.Command {

	var logToStderr bool
	var outDir string
	var overlaysDir string
	var quiet bool
	var verbose int
	var profile string
	var heapProfile string
	var tracePath string
	var debug bool
	var skipDocs bool
	var skipExamples bool
	cmd := &cobra.Command{
		Use:   os.Args[0] + " <LANGUAGE>",
		Args:  cmdutil.SpecificArgs([]string{"language"}),
		Short: "The Pulumi TFGen compiler generates Pulumi package metadata from a Terraform provider",
		Long: "The Pulumi TFGen compiler generates Pulumi package metadata from a Terraform provider.\n" +
			"\n" +
			"The tool will load the provider from your $PATH, inspect its contents dynamically,\n" +
			"and generate all of the Pulumi metadata necessary to consume the resources.\n" +
			"\n" +
			"<LANGUAGE> indicates which language/runtime to target; the current supported set of\n" +
			"languages is " + fmt.Sprintf("%v", AllLanguages) + ".\n" +
			"\n" +
			"Note that there is no custom Pulumi provider code required, because the generated\n" +
			"provider plugin is metadata-driven and thus works against all Terraform providers.\n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if profile != "" {
				f, err := os.Create(profile)
				if err != nil {
					return err
				}
				if err = pprof.StartCPUProfile(f); err != nil {
					return err
				}
				defer pprof.StopCPUProfile()
			}

			if heapProfile != "" {
				defer func() {
					f, err := os.Create(heapProfile)
					if err != nil {
						log.Printf("could not write heap profile: %v", err)
						return
					}
					runtime.GC() // get up-to-date statistics
					if err := pprof.WriteHeapProfile(f); err != nil {
						log.Printf("could not write heap profile: %v", err)
					}
				}()
			}

			if tracePath != "" {
				f, err := os.Create(tracePath)
				if err != nil {
					return err
				}
				if err = trace.Start(f); err != nil {
					return err
				}
				defer trace.Stop()
			}

			// Create the output directory.
			var root afero.Fs
			if outDir != "" {
				absOutDir, err := filepath.Abs(outDir)
				if err != nil {
					return err
				}
				if err = os.MkdirAll(absOutDir, 0700); err != nil {
					return err
				}
				root = afero.NewBasePathFs(afero.NewOsFs(), absOutDir)
			}

			// Creating an item to keep track of example coverage if the
			// COVERAGE_OUTPUT_DIR env is set
			var coverageTracker *CoverageTracker
			coverageOutputDir, coverageTrackingOutputEnabled := os.LookupEnv("COVERAGE_OUTPUT_DIR")
			coverageTracker = newCoverageTracker(prov.Name, prov.Version)

			opts := GeneratorOptions{
				Package:         pkg,
				Version:         version,
				Language:        Language(args[0]),
				ProviderInfo:    prov,
				Root:            root,
				Debug:           debug,
				SkipDocs:        skipDocs,
				SkipExamples:    skipExamples,
				CoverageTracker: coverageTracker,
			}

			err := gen(opts)

			// Exporting collected coverage data to the directory specified by COVERAGE_OUTPUT_DIR
			if coverageTrackingOutputEnabled {
				err = coverageTracker.exportResults(coverageOutputDir)
			} else {
				fmt.Println("\nAdditional example conversion stats are available by setting COVERAGE_OUTPUT_DIR.")
			}
			fmt.Println(coverageTracker.getShortResultSummary())
			printDocStats()

			return err
		}),
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(
		&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().StringVarP(
		&outDir, "out", "o", "", "Emit the generated SDK to this directory")
	cmd.PersistentFlags().BoolVarP(
		&quiet, "quiet", "q", false, "Suppress non-error output progress messages")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")
	cmd.PersistentFlags().StringVar(
		&profile, "profile", "", "Write a CPU profile to this file")
	cmd.PersistentFlags().StringVar(
		&heapProfile, "heap-profile", "", "Write a heap profile to this file")
	cmd.PersistentFlags().StringVar(
		&tracePath, "trace", "", "Write a Go runtime trace to this file")
	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false, "Enable debug logging")
	cmd.PersistentFlags().BoolVar(
		&skipDocs, "skip-docs", false, "Do not convert docs from TF Markdown")
	cmd.PersistentFlags().BoolVar(
		&skipExamples, "skip-examples", false, "Do not convert examples from HCL")

	cmd.PersistentFlags().StringVar(
		&overlaysDir, "overlays", "",
		"Use the target directory for overlays rather than the default of overlays/ (unsupported)")
	err := cmd.PersistentFlags().MarkHidden("overlays")
	contract.AssertNoErrorf(err, "err != nil")

	return cmd
}
