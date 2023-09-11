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

package convert

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	yaml "gopkg.in/yaml.v3"

	"github.com/blang/semver"
	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLoader struct {
	path string
}

func (l *testLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	schemaPath := pkg
	if version != nil {
		schemaPath += "-" + version.String()
	}
	schemaPath = filepath.Join(l.path, schemaPath) + ".json"

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}

	var spec schema.PackageSpec
	err = json.Unmarshal(schemaBytes, &spec)
	if err != nil {
		return nil, err
	}

	schemaPackage, diags, err := schema.BindSpec(spec, l)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}

	return schemaPackage, nil
}

func (l *testLoader) LoadPackageReference(pkg string, version *semver.Version) (schema.PackageReference, error) {
	schemaPackage, err := l.LoadPackage(pkg, version)
	if err != nil {
		return nil, err
	}
	return schemaPackage.Reference(), nil
}

// applyPragmas parses the text from `src` and writes the resulting text to `dst`. It can exclude blocks of
// text based on pragmas and the `isExperimental` flag.
// Pragmas should take the form of:
// #if EXPERIMENTAL
//
//	/* for experimental code */
//
// #else
//
//	/* for normal code */
//
// #endif
//
// The #else block is optional.
func applyPragmas(src io.Reader, dst io.Writer, isExperimental bool) error {
	scanner := bufio.NewScanner(src)
	inIf := false
	inElse := false
	for scanner.Scan() {
		line := scanner.Text()
		// We don't want to test against lines with whitespace, that is "   #else", "  #else ", and "#else "
		// should all be seen as an "#else" line, but when we write the line to `dst` we want to maintain it's
		// whitespace.
		trimLine := strings.TrimSpace(scanner.Text())

		if trimLine == "#if EXPERIMENTAL" {
			if inIf || inElse {
				return fmt.Errorf("Saw #if while already in an if or else block")
			}
			inIf = true
			continue
		}
		if trimLine == "#else" {
			if !inIf {
				return fmt.Errorf("Saw #else while not in an if block")
			}
			if inElse {
				return fmt.Errorf("Saw #else while already in an else block")
			}
			inIf = false
			inElse = true
			continue
		}
		if trimLine == "#endif" {
			if !(inElse || inIf) {
				return fmt.Errorf("Saw #endif while not in an if or else block")
			}
			inIf = false
			inElse = false
			continue
		}

		doWrite := (inIf && isExperimental) ||
			(inElse && !isExperimental) ||
			(!inIf && !inElse)

		if doWrite {
			_, err := dst.Write([]byte(line + "\n"))
			if err != nil {
				return err
			}
		}
	}
	if inIf {
		return fmt.Errorf("Unclosed #if statement")
	}
	if inElse {
		return fmt.Errorf("Unclosed #else statement")
	}
	return scanner.Err()
}

// TestEject runs through all the folders in testdata (except for "schemas" and "mappings") and tries to
// convert all the .tf files in that folder into PCL.
//
// It will use schemas from the testdata/schemas folder, and mappings from the testdata/mappings folder. The
// resulting PCL will be checked against PCL written to a subfolder inside each test folder called "pcl".
//
// The .tf code for each test can also contain pragma comments, see the "applyPragmas" function for
// information about pragmas.
//
// These tests can also be run with PULUMI_EXPERIMENTAL=1, in which case the resulting pcl is checked against
// a folder "experimental_pcl".
func TestEject(t *testing.T) {
	// Test framework for eject
	// Each folder in testdata has a pcl folder, we check that if we convert the hcl we get the expected pcl
	// You can regenerate the test data by running "PULUMI_ACCEPT=1 go test" in this folder (pkg/tf2pulumi/convert).
	testDir, err := filepath.Abs(filepath.Join("testdata"))
	require.NoError(t, err)
	infos, err := os.ReadDir(filepath.Join(testDir, "programs"))
	require.NoError(t, err)

	tests := make([]struct {
		name string
		path string
	}, 0)
	for _, info := range infos {
		if info.IsDir() {
			tests = append(tests, struct {
				name string
				path string
			}{
				name: info.Name(),
				path: filepath.Join(testDir, "programs", info.Name()),
			})
		}
	}

	loader := &testLoader{path: filepath.Join(testDir, "schemas")}
	mapper := &bridgetesting.TestFileMapper{Path: filepath.Join(testDir, "mappings")}

	for _, tt := range tests {
		tt := tt // avoid capturing loop variable in the closure

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isExperimental := isTruthy(os.Getenv("PULUMI_EXPERIMENTAL"))

			pclPath := filepath.Join(tt.path, "pcl")
			// We want to support running this in experimental mode as well as compatibility mode
			if isExperimental {
				pclPath = filepath.Join(tt.path, "experimental_pcl")
			}
			if isTruthy(os.Getenv("PULUMI_ACCEPT")) {
				err := os.RemoveAll(pclPath)
				require.NoError(t, err, "failed to remove existing files at %s", pclPath)
				err = os.MkdirAll(pclPath, 0700)
				require.NoError(t, err, "failed to create directory at %s", pclPath)
			}

			// Copy the .tf files to a new directory and fix up any "#if EXPERIMENTAL/#else/#endif" sections
			tempDir := t.TempDir()
			hclPath := filepath.Join(tempDir, tt.name)
			modulePath := filepath.Join(tempDir, "modules")

			copy := func(srcDirectory, dstDirectory string) {
				err = filepath.WalkDir(srcDirectory, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}

					if strings.HasSuffix(d.Name(), ".tf") {
						src, err := os.Open(path)
						if err != nil {
							return fmt.Errorf("could not open src: %w", err)
						}
						defer src.Close()

						relativePath, err := filepath.Rel(srcDirectory, path)
						if err != nil {
							return err
						}

						dstPath := filepath.Join(dstDirectory, relativePath)
						dstDir := filepath.Dir(dstPath)
						err = os.MkdirAll(dstDir, 0750)
						if err != nil {
							return fmt.Errorf("could not create dst dir: %w", err)
						}

						dst, err := os.Create(dstPath)
						if err != nil {
							return fmt.Errorf("could not open dst: %w", err)
						}
						defer dst.Close()

						err = applyPragmas(src, dst, isExperimental)
						if err != nil {
							return err
						}
					}

					return nil
				})
				require.NoError(t, err)
			}

			copy(tt.path, hclPath)
			copy(filepath.Join(testDir, "modules"), modulePath)

			// If this is a partial test turn on the options to allow missing bits
			partial := strings.HasPrefix(tt.name, "partial_")
			var setOpts func(*EjectOptions)
			if partial {
				setOpts = func(opts *EjectOptions) {
					opts.SkipResourceTypechecking = true
					opts.AllowMissingProperties = true
					opts.AllowMissingVariables = true
					opts.FilterResourceNames = true
				}
			}

			project, program, ejectErr := ejectWithOpts(hclPath, loader, mapper, setOpts)

			// Check if this is an error test
			var expectedError string
			expectedErrorPath := filepath.Join(pclPath, "error.txt")

			// If PULUMI_ACCEPT is set write the errors to the folder
			if isTruthy(os.Getenv("PULUMI_ACCEPT")) {
				// If ejectErr is nil write nothing
				if ejectErr != nil {
					errorText := ejectErr.Error()
					// Replace any instances of the temp directory with a fixed string
					errorText = strings.ReplaceAll(errorText, tempDir, "<tempDir>")
					err = os.WriteFile(expectedErrorPath, []byte(errorText), 0600)
					require.NoError(t, err, "failed to write error")
				}
			}

			expectedErrorBytes, err := os.ReadFile(expectedErrorPath)
			if !os.IsNotExist(err) {
				require.NoError(t, err)
				expectedError = string(expectedErrorBytes)
			}

			if expectedError != "" {
				errorText := ejectErr.Error()
				// Again replace any instances of the temp directory with a fixed string
				errorText = strings.ReplaceAll(errorText, tempDir, "<tempDir>")
				assert.Equal(t, expectedError, errorText)
			} else {
				require.NoError(t, ejectErr)
				// Assert the project name is as expected (the directory name)
				assert.Equal(t, tokens.PackageName(tt.name), project.Name)

				// Assert every pcl file is seen
				_, err = os.ReadDir(pclPath)
				if !os.IsNotExist(err) && !assert.NoError(t, err) {
					// If the directory was not found then the expected pcl results are the empty set, but if the
					// directory could not be read because of filesystem issues than just error out.
					assert.FailNow(t, "Could not read expected pcl results")
				}

				writeToFileSystem := func(fs afero.Fs) {
					err = program.WriteSource(fs)
					require.NoError(t, err, "failed to write program source files")
					// If the project has any config write that to the memory file system
					if len(project.Config) > 0 {
						buffer, err := yaml.Marshal(project)
						require.NoError(t, err, "failed to marshal project config")
						err = afero.WriteFile(fs, "/Pulumi.yaml", buffer, 0600)
						require.NoError(t, err, "failed to write Pulumi.yaml")
					}
				}

				// If PULUMI_ACCEPT is set then clear the PCL folder and write the generated files out
				pclFs := afero.NewBasePathFs(afero.NewOsFs(), pclPath)
				if isTruthy(os.Getenv("PULUMI_ACCEPT")) {
					writeToFileSystem(pclFs)
				}

				// Write the program to a memory file system
				pclMemFs := afero.NewMemMapFs()
				writeToFileSystem(pclMemFs)

				// compare the generated files with files on disk
				err = afero.Walk(pclMemFs, "/", func(path string, info fs.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info == nil || info.IsDir() {
						// ignore directories
						return nil
					}

					sourceOnDisk, err := afero.ReadFile(pclFs, path)
					assert.NoError(t, err, "generated source file must be on disk")
					sourceInMemory, err := afero.ReadFile(pclMemFs, path)
					assert.NoError(t, err, "should be able to read %s", path)
					expectedPcl := strings.Replace(string(sourceOnDisk), "\r\n", "\n", -1)
					actualPcl := strings.Replace(string(sourceInMemory), "\r\n", "\n", -1)
					assert.Equal(t, expectedPcl, actualPcl)
					return nil
				})
				require.NoError(t, err, "failed to check written files")

				// make sure _all_ files on disk are also generated in the source
				err = afero.Walk(pclFs, "/", func(path string, info fs.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info == nil || info.IsDir() {
						// ignore directories and non-PCL files
						return nil
					}

					_, err = afero.ReadFile(pclMemFs, path)
					assert.NoError(t, err, "file on disk was not generated in memory: %s", path)
					return nil
				})
				// It's ok for the pcl directory to just not exist, this happens for the empty tests.
				if !errors.Is(err, fs.ErrNotExist) {
					require.NoError(t, err, "failed to check saved files")
				}
			}
		})
	}
}
