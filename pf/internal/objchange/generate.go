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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:generate go run generate.go

const terraformPluginGoRepo = "https://github.com/hashicorp/terraform-plugin-go.git"
const terraformPluginGoVer = "v0.15.0"

type file struct {
	remote     string
	src        string
	dest       string
	transforms []func(string) string
}

func main() {
	for _, path := range []string{"toproto"} {
		if err := os.RemoveAll(path); err != nil {
			panic(err)
		}
	}

	terraformPluginGoRemote := fetchRemote("terraform-plugin-go", terraformPluginGoRepo, terraformPluginGoVer)
	fmt.Println(terraformPluginGoRemote)

	remotes := map[string]string{
		"terraform-plugin-go": terraformPluginGoRemote,
	}

	for _, f := range files() {
		install(remotes, f)
	}
}

func files() []file {
	replaceProtoRef := gofmtReplace(
		fmt.Sprintf(`"%s" -> "%s"`,
			"github.com/hashicorp/terraform-plugin-go/tfprotov6/internal/tfplugin6",
			"github.com/pulumi/terraform/pkg/tfplugin6"))

	tweakProtoMetadata := gofmtReplace(fmt.Sprintf(`"%s" -> "%s"`, "tfplugin6.proto", "tfplugin6x.proto"))

	transforms := []func(string) string{
		replaceProtoRef,
		tweakProtoMetadata,
	}

	files := []file{
		{
			remote: "terraform-plugin-go",
			src:    "LICENSE",
			dest:   "toproto/LICENSE",
		},
		{
			remote:     "terraform-plugin-go",
			src:        "tfprotov6/internal/toproto/schema.go",
			dest:       "toproto/schema.go",
			transforms: transforms,
		},
		{
			remote:     "terraform-plugin-go",
			src:        "tfprotov6/internal/toproto/string_kind.go",
			dest:       "toproto/string_kind.go",
			transforms: transforms,
		},
		{
			remote:     "terraform-plugin-go",
			src:        "tfprotov6/internal/toproto/dynamic_value.go",
			dest:       "toproto/dynamic_value.go",
			transforms: transforms,
		},
	}

	for i, f := range files {
		t := doNotEditWarning(f.remote)
		if strings.HasSuffix(f.dest, "LICENSE") {
			continue
		}
		files[i].transforms = append(files[i].transforms, t)
	}

	return files
}

func install(remotes map[string]string, f file) {
	srcPath := filepath.Join(remotes[f.remote], filepath.Join(strings.Split(f.src, "/")...))
	code, err := os.ReadFile(srcPath)
	if err != nil {
		panic(err)
	}
	for _, t := range f.transforms {
		code = []byte(t(string(code)))
	}
	destPath := filepath.Join(strings.Split(f.dest, "/")...)
	ensureDirFor(destPath)
	if err := os.WriteFile(destPath, code, os.ModePerm); err != nil {
		panic(err)
	}
}

func ensureDirFor(path string) {
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		panic(err)
	}
}

func fetchRemote(prefix, repo, ver string) string {
	tmp := os.TempDir()
	dir := filepath.Join(tmp, prefix+"-"+ver)
	stat, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if os.IsNotExist(err) || !stat.IsDir() {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			panic(err)
		}
		cmd := exec.Command("git", "clone", "-b", ver, repo, dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			panic(err)
		}
	}
	return dir
}

func gofmtReplace(spec string) func(string) string {
	return func(code string) string {
		t, err := os.CreateTemp("", "gofmt*.go")
		if err != nil {
			panic(err)
		}
		defer os.Remove(t.Name())
		if err := os.WriteFile(t.Name(), []byte(code), os.ModePerm); err != nil {
			panic(err)
		}
		var stdout bytes.Buffer
		cmd := exec.Command("gofmt", "-r", spec, t.Name())
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			panic(err)
		}
		return stdout.String()
	}
}

func doNotEditWarning(remote string) func(code string) string {
	src := map[string]string{
		"terraform-plugin-go": terraformPluginGoRepo,
	}
	return func(code string) string {
		return "// Code copied from " + src[remote] + " by go generate; DO NOT EDIT.\n" + code
	}
}
