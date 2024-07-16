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

//go:generate go run generate.go

const (
	oldPkg  = "github.com/hashicorp/terraform-plugin-go"
	newPkg  = "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/terraform-plugin-go"
	tpgRepo = "https://github.com/hashicorp/terraform-plugin-go"
	tpgVer  = "v0.22.0"
)

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
	fixupTFPlugin6Ref := gofmtReplace(fmt.Sprintf(
		`"%s" -> "%s"`,
		fmt.Sprintf("%s/tfprotov6/internal/tfplugin6", oldPkg),
		fmt.Sprintf("%s/tfprotov6/tfplugin6", newPkg),
	))

	transforms := []func(string) string{
		fixupTFPlugin6Ref,
	}

	return []file{
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
		{
			src:        "tfprotov6/internal/tfplugin6/tfplugin6.pb.go",
			dest:       "tfprotov6/tfplugin6/tfplugin6.pb.go",
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
	dir := filepath.Join(tmp, "terraform-plugin-go-"+tpgVer)
	stat, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	if os.IsNotExist(err) || !stat.IsDir() {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		cmd := exec.Command("git", "clone", "-b", tpgVer, tpgRepo, dir)
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
	return "// Code copied from " + tpgRepo + " by go generate; DO NOT EDIT.\n" + code
}

func fixupCodeTypeError(code string) string {
	before := `panic(fmt.Sprintf("unsupported block nesting mode %s"`
	after := `panic(fmt.Sprintf("unsupported block nesting mode %v"`
	return strings.ReplaceAll(code, before, after)
}
