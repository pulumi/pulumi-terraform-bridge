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
)

//go:generate go run generate.go

const terraformRepo = "https://github.com/hashicorp/terraform.git"
const terraformVer = "v1.3.9"

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
	oldPkg := "github.com/hashicorp/terraform"
	newPkg := "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/objchange"

	replacePkg := gofmtReplace(fmt.Sprintf(`"%s/internal/configs/configschema" -> "%s/configs/configschema"`,
		oldPkg, newPkg))

	return []file{
		{
			src:  "internal/configs/configschema/schema.go",
			dest: "configs/configschema/schema.go",
		},
		{
			src:  "internal/configs/configschema/empty_value.go",
			dest: "configs/configschema/empty_value.go",
		},
		{
			src:  "internal/configs/configschema/implied_type.go",
			dest: "configs/configschema/implied_type.go",
		},
		{
			src:  "internal/configs/configschema/decoder_spec.go",
			dest: "configs/configschema/decoder_spec.go",
		},
		{
			src:  "internal/plans/objchange/objchange.go",
			dest: "plans/objchange/objchange.go",
			transforms: []func(string) string{
				replacePkg,
			},
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
