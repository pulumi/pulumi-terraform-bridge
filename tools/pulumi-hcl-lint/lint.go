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

package main

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/pulumi/terraform/pkg/configs"
	"github.com/spf13/afero"
)

func lint(ctx context.Context, dir string, sink chan<- issue) error {
	fs, err := configureFS(dir)
	if err != nil {
		return err
	}
	mod, err := loadModule(fs)
	if err != nil {
		return err
	}
	return lintModule(ctx, mod, sink)
}

func configureFS(dir string) (afero.Fs, error) {
	absPathHere, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	fs := afero.NewBasePathFs(afero.NewOsFs(), absPathHere)
	return fs, nil
}

func loadModule(fs afero.Fs) (*configs.Module, error) {
	path := "."
	p := configs.NewParser(fs)
	mod, diags := p.LoadConfigDir(path)
	if diags.Errs() != nil {
		return nil, errors.Join(diags.Errs()...)
	}
	return mod, nil
}

func lintModule(ctx context.Context, mod *configs.Module, sink chan<- issue) error {
	lintDanglingRefs(ctx, mod, sink)
	return nil
}
