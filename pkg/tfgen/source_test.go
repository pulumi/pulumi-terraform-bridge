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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestGetDocsPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		files              []string
		expectedResource   []string
		expectedDataSource []string
	}{
		{files: []string{"main.go"}}, // No docs files => no valid paths

		{
			files: []string{
				filepath.Join("website", "docs", "r", "foo.md"),
				filepath.Join("website", "docs", "d", "bar.md"),
			},
			expectedResource:   []string{filepath.Join("website", "docs", "r")},
			expectedDataSource: []string{filepath.Join("website", "docs", "d")},
		},
		{
			files: []string{
				filepath.Join("docs", "resources", "foo.md"),
				filepath.Join("website", "docs", "d", "bar.md"),
			},
			expectedResource:   []string{filepath.Join("docs", "resources")},
			expectedDataSource: []string{filepath.Join("website", "docs", "d")},
		},
		{
			files: []string{
				filepath.Join("docs", "resources", "foo.md"),
				filepath.Join("website", "docs", "r", "bar.md"),
			},
			expectedResource: []string{
				filepath.Join("website", "docs", "r"),
				filepath.Join("docs", "resources"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()

			repo := t.TempDir()
			for _, f := range tt.files {
				p := filepath.Join(repo, f)
				require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
				require.NoError(t, os.WriteFile(p, []byte("test"), 0o600))
			}

			check := func(expected, actual []string, err error) {
				if !assert.NoError(t, err) {
					return
				}
				for i, v := range expected {
					expected[i] = filepath.Join(repo, v)
				}
				assert.Equal(t, expected, actual)
			}

			actualResource, err := getDocsPath(repo, ResourceDocs)
			check(tt.expectedResource, actualResource, err)

			actualDataSource, err := getDocsPath(repo, DataSourceDocs)
			check(tt.expectedDataSource, actualDataSource, err)
		})
	}
}

func TestGetNarkdownNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		packagePrefix string
		rawName       string
		globalInfo    *tfbridge.DocRuleInfo
		expectedNames []string
	}{
		{
			name:          "Generates collection of possible markdown names",
			packagePrefix: "mongodbatlas",
			rawName:       "mongodbatlas_x509_authentication_database_user",
			globalInfo:    nil,
			expectedNames: []string{
				"x509_authentication_database_user.html.markdown",
				"x509_authentication_database_user.markdown",
				"x509_authentication_database_user.html.md",
				"x509_authentication_database_user.md",
				"mongodbatlas_x509_authentication_database_user.html.markdown",
				"mongodbatlas_x509_authentication_database_user.markdown",
				"mongodbatlas_x509_authentication_database_user.html.md",
				"mongodbatlas_x509_authentication_database_user.md",
			},
		},
		{
			name:          "Trims tfbridge.RenamedEntitySuffix from possible markdown names",
			packagePrefix: "mongodbatlas",
			rawName:       "mongodbatlas_x509_authentication_database_user_legacy",
			globalInfo:    nil,
			expectedNames: []string{
				"x509_authentication_database_user.html.markdown",
				"x509_authentication_database_user.markdown",
				"x509_authentication_database_user.html.md",
				"x509_authentication_database_user.md",
				"mongodbatlas_x509_authentication_database_user.html.markdown",
				"mongodbatlas_x509_authentication_database_user.markdown",
				"mongodbatlas_x509_authentication_database_user.html.md",
				"mongodbatlas_x509_authentication_database_user.md",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actualNames := getMarkdownNames(tt.packagePrefix, tt.rawName, tt.globalInfo)
			assert.Equal(t, tt.expectedNames, actualNames)
		})
	}
}
