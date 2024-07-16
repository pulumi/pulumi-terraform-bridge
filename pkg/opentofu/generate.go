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

//go:build generate
// +build generate

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

//go:generate go run generate.go

const terraformRepo = "https://github.com/opentofu/opentofu.git"
const terraformVer = "v1.7.2"

type file struct {
	src        string
	dest       string
	transforms []func(string) string
}

func main() {
	remote := fetchRemote()
	fmt.Println(remote)

	for _, f := range files() {
		install(remote, f)
	}
}

func files() []file {
	oldPkg := "github.com/opentofu/opentofu"
	newPkg := "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/opentofu"

	replacePkg := gofmtReplace(fmt.Sprintf(`"%s/internal/configs/configschema" -> "%s/configs/configschema"`,
		oldPkg, newPkg))

	transforms := []func(string) string{
		replacePkg,
		doNotEditWarning,
		fixupCodeTypeError,
	}

	return []file{
		{
			src:  "LICENSE",
			dest: "configs/configschema/LICENSE",
		},
		{
			src:  "LICENSE",
			dest: "plans/objchange/LICENSE",
		},
		{
			src:        "internal/configs/configschema/schema.go",
			dest:       "configs/configschema/schema.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/empty_value.go",
			dest:       "configs/configschema/empty_value.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/implied_type.go",
			dest:       "configs/configschema/implied_type.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/decoder_spec.go",
			dest:       "configs/configschema/decoder_spec.go",
			transforms: transforms,
		},
		{
			src:        "internal/configs/configschema/path.go",
			dest:       "configs/configschema/path.go",
			transforms: transforms,
		},
		{
			src:        "internal/plans/objchange/objchange.go",
			dest:       "plans/objchange/objchange.go",
			transforms: append(transforms, patchProposedNewForUnknownBlocks),
		},
		{
			src:        "internal/plans/objchange/plan_valid.go",
			dest:       "plans/objchange/plan_valid.go",
			transforms: transforms,
		},
	}
}

func install(remote string, f file) {
	srcPath := filepath.Join(remote, filepath.Join(strings.Split(f.src, "/")...))
	code, err := os.ReadFile(srcPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range f.transforms {
		code = []byte(t(string(code)))
	}
	destPath := filepath.Join(strings.Split(f.dest, "/")...)
	ensureDirFor(destPath)
	if err := os.WriteFile(destPath, code, os.ModePerm); err != nil {
		log.Fatal(err)
	}
}

func ensureDirFor(path string) {
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}

func fetchRemote() string {
	tmp := os.TempDir()
	dir := filepath.Join(tmp, "terraform-"+terraformVer)
	stat, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	if os.IsNotExist(err) || !stat.IsDir() {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		cmd := exec.Command("git", "clone", "-b", terraformVer, terraformRepo, dir)
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
	return dir
}

func gofmtReplace(spec string) func(string) string {
	return func(code string) string {
		t, err := os.CreateTemp("", "gofmt*.go")
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove(t.Name())
		if err := os.WriteFile(t.Name(), []byte(code), os.ModePerm); err != nil {
			log.Fatal(err)
		}
		var stdout bytes.Buffer
		cmd := exec.Command("gofmt", "-r", spec, t.Name())
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
		return stdout.String()
	}
}

func doNotEditWarning(code string) string {
	return "// Code copied from " + terraformRepo + " by go generate; DO NOT EDIT.\n" + code
}

func fixupCodeTypeError(code string) string {
	before := `panic(fmt.Sprintf("unsupported block nesting mode %s"`
	after := `panic(fmt.Sprintf("unsupported block nesting mode %v"`
	return strings.ReplaceAll(code, before, after)
}

// This patch introduces a change in behavior for the vendored objchange.ProposedNew algorithm. Before the change,
// planning a block change where config is entirely unknown used to pick the prior state. After the change it picks the
// unknown. This is especially interesting when planning set-nested blocks, as when the algorithm fails to find a
// matching set element in priorState it will send prior=null instead, and proceed to substitute null with an empty
// value matching the block structure. Without the patch, this empty value will be selected over the unknown and
// surfaced to Pulumi users, which is confusing.
//
// See TestUnknowns test suite and the "unknown for set block prop" test case.
//
// TODO[pulumi/pulumi-terraform-bridge#2247] revisit this patch.
func patchProposedNewForUnknownBlocks(goCode string) string {
	oldCode := `func proposedNew(schema *configschema.Block, prior, config cty.Value) cty.Value {
	if config.IsNull() || !config.IsKnown() {`

	newCode := `func proposedNew(schema *configschema.Block, prior, config cty.Value) cty.Value {
	if !config.IsKnown() {
		return config
	}
	if config.IsNull() {`
	updatedGoCode := strings.Replace(goCode, oldCode, newCode, 1)
	contract.Assertf(updatedGoCode != oldCode, "patchProposedNewForUnknownBlocks failed to apply")
	return updatedGoCode
}
