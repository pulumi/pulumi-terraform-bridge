// Copyright 2016-2019, Pulumi Corporation.
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
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
)

func LoadGoMod() (*modfile.File, error) {
	exePath, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "error determining working directory")
	}

	moduleRoot := findModuleRoot(exePath)
	if moduleRoot == "" {
		// Some provider repos have a "provider" module, rather than a
		// module at the root of the repo.
		moduleRoot = findModuleRoot(exePath + "/provider")
		if moduleRoot == "" {
			return nil, errors.New("cannot find module root")
		}
	}

	gomodContent, err := ioutil.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return nil, errors.Wrap(err, "error reading go.mod")
	}

	file, err := modfile.Parse("go.mod", gomodContent, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing go.mod")
	}

	return file, nil
}

func FindEffectiveModuleForImportPath(file *modfile.File, importPath string) (string, string, error) {
	// If the module has been replaced, we need the replacement path and version
	for _, rep := range file.Replace {
		if rep.Old.Path != importPath {
			continue
		}

		return rep.New.Path, rep.New.Version, nil
	}

	// Otherwise find the required version
	for _, req := range file.Require {
		if req.Mod.Path != importPath {
			continue
		}

		return req.Mod.Path, req.Mod.Version, nil
	}

	return "", "", fmt.Errorf("%q does not appear to be required by go.mod", importPath)
}

func GetModuleRoot(repo string, version string) string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	baseAndVersion := fmt.Sprintf("%s@%s", filepath.Base(repo), version)
	return filepath.Join(gopath, "pkg", "mod", filepath.Dir(repo), baseAndVersion)
}

// Copyright 2018 The Go Authors. - Taken from src/cmd/go/internal/modload/init.go
func findModuleRoot(dir string) (root string) {
	dir = filepath.Clean(dir)

	// Look for enclosing go.mod.
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		d := filepath.Dir(dir)
		if d == dir {
			break
		}
		dir = d
	}
	return ""
}
