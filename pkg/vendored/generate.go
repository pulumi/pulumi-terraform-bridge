// Copyright 2016-2024, Pulumi Corporation.
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

//go:generate go run generate.go

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	vendorTerraformPluginGo("v0.22.0")
}

func vendorTerraformPluginGo(version string) {
	oldPkg := "github.com/hashicorp/terraform-plugin-go"
	protoPkg := "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"

	fixupTFPlugin6Ref := gofmtReplace(fmt.Sprintf(
		`"%s" -> "%s"`,
		fmt.Sprintf("%s/tfprotov6/internal/tfplugin6", oldPkg),
		protoPkg,
	))

	doNotEditWarning := func(code string) string {
		return fmt.Sprintf("// Code copied from %s by go generate; DO NOT EDIT.\n", oldPkg) + code
	}

	fixupCodeTypeError := func(code string) string {
		before := `panic(fmt.Sprintf("unsupported block nesting mode %s"`
		after := `panic(fmt.Sprintf("unsupported block nesting mode %v"`
		return strings.ReplaceAll(code, before, after)
	}

	transforms := []func(string) string{
		doNotEditWarning,
		fixupTFPlugin6Ref,
		fixupCodeTypeError,
	}

	files := []file{
		{
			src:  "LICENSE",
			dest: "tfprotov6/LICENSE",
		},
		{
			src:        "tfprotov6/internal/toproto/schema.go",
			dest:       "tfprotov6/toproto/schema.go",
			transforms: transforms,
		},
		{
			src:        "tfprotov6/internal/toproto/string_kind.go",
			dest:       "tfprotov6/toproto/string_kind.go",
			transforms: transforms,
		},
		{
			src:        "tfprotov6/internal/toproto/dynamic_value.go",
			dest:       "tfprotov6/toproto/dynamic_value.go",
			transforms: transforms,
		},
	}

	vendor(vendorOpts{
		repo:      "https://github.com/hashicorp/terraform-plugin-go",
		version:   version,
		files:     files,
		targetDir: "terraform-plugin-go",
	})
}

type file struct {
	src        string
	dest       string
	transforms []func(string) string
}

type vendorOpts struct {
	repo      string
	version   string
	files     []file
	targetDir string
}

func vendor(opts vendorOpts) {
	err := os.RemoveAll(opts.targetDir)
	if err != nil {
		log.Fatal(err)
	}
	sources := fetchRemote(opts.repo, opts.version)
	for _, f := range opts.files {
		srcPath := filepath.Join(sources, filepath.Join(strings.Split(f.src, "/")...))
		code, err := os.ReadFile(srcPath)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range f.transforms {
			code = []byte(t(string(code)))
		}
		destPath := filepath.Join(opts.targetDir, filepath.Join(strings.Split(f.dest, "/")...))
		ensureDirFor(destPath)
		if err := os.WriteFile(destPath, code, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}
}

// Resolves a Git repository to a local folder and returns that folder.
//
// Example:
//
//	fetchRemote("https://github.com/hashicorp/terraform-plugin-go", "v0.22.0")
func fetchRemote(repo, version string) string {
	parts := strings.Split(repo, "/")
	lastPart := parts[len(parts)-1]
	tmp := os.TempDir()
	dir := filepath.Join(tmp, lastPart+"-"+version)
	stat, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	if os.IsNotExist(err) || !stat.IsDir() {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		cmd := exec.Command("git", "clone", "-b", version, repo, dir)
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

func ensureDirFor(path string) {
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}
