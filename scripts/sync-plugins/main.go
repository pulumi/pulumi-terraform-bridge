// Copyright 2026, Pulumi Corporation.
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

// Rebuild the repo-local Pulumi plugin directory from mise's selected plugins.
// Run by mise's postinstall hook, before starting tests, never during a test run.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

func main() {
	if err := syncPlugins(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func syncPlugins() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	config, err := filepath.EvalSymlinks(filepath.Join(root, "mise.toml"))
	if err != nil {
		return err
	}
	root = filepath.Dir(config)
	listing := exec.Command("mise", "ls", "--current", "--json")
	listing.Stderr = os.Stderr
	data, err := listing.Output()
	if err != nil {
		return fmt.Errorf("list mise plugins: %w", err)
	}
	var tools map[string][]struct {
		Version     string `json:"version"`
		Requested   string `json:"requested_version"`
		InstallPath string `json:"install_path"`
		Installed   bool   `json:"installed"`
		Source      struct {
			Path string `json:"path"`
		} `json:"source"`
	}
	if err := json.Unmarshal(data, &tools); err != nil {
		return fmt.Errorf("decode mise plugins: %w", err)
	}
	type plugin struct{ directory, binary, source string }
	var plugins []plugin
	for _, tool := range slices.Sorted(maps.Keys(tools)) {
		if !strings.HasPrefix(tool, "vfox-pulumi:") {
			continue
		}
		for _, pin := range tools[tool] {
			source, err := filepath.EvalSymlinks(pin.Source.Path)
			if err != nil || source != config {
				continue
			}
			if !pin.Installed || pin.Version != pin.Requested {
				return fmt.Errorf("plugin %s must be installed at its exact pin, run mise install", tool)
			}
			name := strings.TrimPrefix(tool[strings.LastIndex(tool, "/")+1:], "pulumi-")
			kind := "resource"
			if converter, ok := strings.CutPrefix(name, "converter-"); ok {
				kind, name = "converter", converter
			}
			binary := "pulumi-" + kind + "-" + name
			if runtime.GOOS == "windows" {
				binary += ".exe"
			}
			path := filepath.Join(pin.InstallPath, "bin", binary)
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("missing mise plugin %s: %w", tool, err)
			}
			plugins = append(plugins, plugin{kind + "-" + name + "-v" + pin.Version, binary, path})
		}
	}
	if len(plugins) == 0 {
		return errors.New("no repo-owned mise plugins found")
	}

	// Only reset this checkout's plugin directory. Preserve credentials, state,
	// and the dynamic bridge's separate Terraform provider cache in .pulumi.
	home := filepath.Join(root, ".pulumi")
	if info, err := os.Lstat(home); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to reset plugins through symlink %s", home)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	cache := filepath.Join(home, "plugins")
	if err := os.RemoveAll(cache); err != nil {
		return err
	}
	bin := filepath.Join(cache, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	for _, p := range plugins {
		dir := filepath.Join(cache, p.directory)
		if err := os.Mkdir(dir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(dir, p.binary)
		if err := linkOrCopy(p.source, path); err != nil {
			return fmt.Errorf("link %s: %w", p.directory, err)
		}
		// Also expose exactly these binaries on PATH. Pulumi supports ambient
		// plugins, so pinning its cache alone would not override inherited binaries.
		if err := os.Link(path, filepath.Join(bin, p.binary)); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "linked %s\n", p.directory)
	}
	return nil
}

func linkOrCopy(source, dest string) error {
	if err := os.Link(source, dest); err == nil {
		return nil
	}
	// Hard links can fail across filesystems. Copy the executable in that case.
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}
