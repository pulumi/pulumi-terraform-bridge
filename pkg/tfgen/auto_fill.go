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

package tfgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/afero"
)

const (
	// Directory to look for stencil HCL resources to auto-fill undeclared resource references.
	//
	// This functionality is activated during HCL->Pulumi example conversion and is aimed at helping provider
	// maintainers improve Pulumi documentation by making it more likely to contain fully working examples.
	//
	// To make things concrete, suppose PULUMI_CONVERT_AUTOFILL_DIR=std_resources and there is an
	// std_resources/aws_acm_certificate.tf file declaring an aws_acm_certificate called "example". In this case
	// documentation generation will splice this definition into all HCL programs that are missing an undeclared
	// resource reference to aws_acm_certificate.
	autoFillEnvVar = "PULUMI_CONVERT_AUTOFILL_DIR"
)

var (
	hclLintPkg = "github.com/pulumi/pulumi-terraform-bridge/tools/pulumi-hcl-lint"
)

// Provides data for [AutoFill].
type autoFillData interface {
	// Returns a suggested automatically filled example HCL code for a given resource or data source name. If this
	// block is not supported or has no plausible examples, returns an empty string.
	AutoFill(token, name string) string

	// Returns true if the given resource or data source token can be passed to AutoFill.
	CanAutoFill(token string) bool
}

type hclResourceRef struct {
	token string
	name  string
}

// Examines an HCL example code snippet to find dangling references to resources or data source calls. When processing
// documentation it is frequently the case that resources are implied but not listed in the original code. If such a
// reference is encountered, this consults autoFiller for a possible canonical definition and augments the program.
func autoFill(autoFiller autoFillData, hcl string) (string, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s\n", hcl)
	refs, err := findDanglingReferences(hcl)
	if err != nil {
		return "", err
	}
	seen := map[string]struct{}{}
	for _, dr := range refs {
		key := fmt.Sprintf("%s:::%s", dr.token, dr.name)
		if _, ok := seen[key]; ok {
			continue
		}
		tok := dr.token
		if !autoFiller.CanAutoFill(tok) {
			continue
		}
		extra := autoFiller.AutoFill(tok, dr.name)
		fmt.Fprintf(&buf, "\n%s\n", extra)
		seen[key] = struct{}{}
	}
	return buf.String(), nil
}

func findDanglingReferences(hcl string) (finalRefs []hclResourceRef, finalErr error) {
	dir, err := os.MkdirTemp("", "example")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil && finalErr == nil {
			finalErr = err
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, "infra.tf"), []byte(hcl), 0600); err != nil {
		return nil, err
	}
	o := filepath.Join(dir, "errors.json")

	// The dependencies on HCL parser are isolated in pulumi-hcl-lint binary, requiring a level of indirection here.
	var cmd *exec.Cmd

	if lint, err := exec.LookPath("pulumi-hcl-lint"); err == nil {
		cmd = exec.Command(lint, "-json", "-out", o)
		cmd.Dir = dir
	} else {
		fmt.Println("!", hclLintPkg)
		cmd = exec.Command("go", "run", hclLintPkg, "-json", "-out", o)
	}
	var stderr, stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed running pulumi-hcl-lint: %v\n%s\n%s\n", err,
			stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(o)
	if err != nil {
		return nil, err
	}
	recs, err := readJsonRecords(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if r["code"] == "PHL0001" {
			token := r["token"].(string)
			name := r["name"].(string)
			finalRefs = append(finalRefs, hclResourceRef{token: token, name: name})
		}
	}
	return finalRefs, nil
}

func readJsonRecords(r io.Reader) ([]map[string]any, error) {
	var records []map[string]any
	dec := json.NewDecoder(r)
	for {
		var record map[string]any
		if err := dec.Decode(&record); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else {
			records = append(records, record)
		}
	}
	return records, nil
}

type folderBasedAutoFiller struct {
	dir afero.Fs
}

var _ autoFillData = (*folderBasedAutoFiller)(nil)

func (fba *folderBasedAutoFiller) AutoFill(token, name string) string {
	bytes, err := afero.ReadFile(fba.dir, fmt.Sprintf("%s.tf", token))
	contract.IgnoreError(err)
	return strings.ReplaceAll(string(bytes), `"example"`, fmt.Sprintf("%q", name))
}

func (fba *folderBasedAutoFiller) CanAutoFill(token string) bool {
	_, err := fba.dir.Stat(fmt.Sprintf("%s.tf", token))
	return err == nil
}

func newAferoAutoFiller(fs afero.Fs) autoFillData {
	return &folderBasedAutoFiller{dir: fs}
}
