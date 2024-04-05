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
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/terraform/pkg/configs"
	"github.com/spf13/afero"
)

// Provides data for [AutoFill].
type autoFillData interface {
	// Returns a suggested automatically filled example HCL code for a given resource or data source name. If this
	// block is not supported or has no plausible examples, returns an empty string.
	AutoFill(token, name string) string

	// Returns true if the given resource or data source token can be passed to AutoFill.
	CanAutoFill(token string) bool
}

// Examines an HCL example code snippet to find dangling references to resources or data source calls. When processing
// documentation it is frequently the case that resources are implied but not listed in the original code. If such a
// reference is encountered, this consults autoFiller for a possible canonical definition and augments the program.
func autoFill(autoFiller autoFillData, hcl string) (string, error) {
	var buf bytes.Buffer
	fs := afero.NewMemMapFs()

	// Create a new file with some content.
	err := afero.WriteFile(fs, "infra.tf", []byte(hcl), 0600)
	if err != nil {
		return "", err
	}

	path := "."
	p := configs.NewParser(fs)
	mod, diags := p.LoadConfigDir(path)
	if diags.Errs() != nil {
		return "", errors.Join(diags.Errs()...)
	}

	v := newAutoFillVisitor()
	for _, mr := range mod.ManagedResources {
		v.visitManagedResource(mr)
	}

	fmt.Fprintf(&buf, "%s\n", hcl)

	for _, dr := range v.dangling() {
		tok := dr.Token()
		if !autoFiller.CanAutoFill(tok) {
			continue
		}
		extra := autoFiller.AutoFill(tok, dr.Name())
		fmt.Fprintf(&buf, "\n%s\n", extra)
	}

	return buf.String(), nil
}

type folderBasedAutoFiller struct {
	dir afero.Fs
}

var _ autoFillData = (*folderBasedAutoFiller)(nil)

func (fba *folderBasedAutoFiller) AutoFill(token, name string) string {
	bytes, err := afero.ReadFile(fba.dir, fmt.Sprintf("%s.tf", token))
	contract.IgnoreError(err)
	return string(bytes)
}

func (fba *folderBasedAutoFiller) CanAutoFill(token string) bool {
	_, err := fba.dir.Stat(fmt.Sprintf("%s.tf", token))
	return err == nil
}

func newAferoAutoFiller(fs afero.Fs) autoFillData {
	return &folderBasedAutoFiller{dir: fs}
}

type autoFillRef string

func (x autoFillRef) Token() string {
	return strings.Split(string(x), ":::")[0]
}

func (x autoFillRef) Name() string {
	return strings.Split(string(x), ":::")[1]
}

func newAutoFillRef(token, name string) autoFillRef {
	return autoFillRef(fmt.Sprintf("%s:::%s", token, name))
}

type autoFillVisitor struct {
	defined    map[autoFillRef]struct{}
	referenced map[autoFillRef]struct{}
}

func newAutoFillVisitor() *autoFillVisitor {
	return &autoFillVisitor{
		defined:    map[autoFillRef]struct{}{},
		referenced: map[autoFillRef]struct{}{},
	}
}

func (v *autoFillVisitor) dangling() []autoFillRef {
	d := []autoFillRef{}
	for x := range v.referenced {
		_, isDef := v.defined[x]
		if !isDef {
			d = append(d, x)
		}
	}
	sort.Slice(d, func(i, j int) bool {
		return string(d[i]) < string(d[j])
	})
	return d
}

func (v *autoFillVisitor) visitManagedResource(res *configs.Resource) {
	v.defined[newAutoFillRef(res.Type, res.Name)] = struct{}{}
	v.visitBody(res.Config)
	v.visitExpr(res.Count)
	v.visitExpr(res.ForEach)
	v.visitTraversals(res.DependsOn)
	v.visitExprs(res.TriggersReplacement)
}

func (v *autoFillVisitor) visitTraversal(t hcl.Traversal) {
	if len(t) < 2 {
		return
	}
	root, ok := t[0].(hcl.TraverseRoot)
	if !ok {
		return
	}
	attr, ok := t[1].(hcl.TraverseAttr)
	if !ok {
		return
	}
	v.referenced[newAutoFillRef(root.Name, attr.Name)] = struct{}{}
}

func (v *autoFillVisitor) visitTraversals(ts []hcl.Traversal) {
	for _, t := range ts {
		v.visitTraversal(t)
	}
}

func (v *autoFillVisitor) visitAttribute(a *hcl.Attribute) {
	v.visitExpr(a.Expr)
}

func (v *autoFillVisitor) visitExpr(expr hcl.Expression) {
	if expr == nil {
		return
	}
	for _, t := range expr.Variables() {
		v.visitTraversal(t)
	}
}

func (v *autoFillVisitor) visitExprs(exprs []hcl.Expression) {
	for _, e := range exprs {
		v.visitExpr(e)
	}
}

func (v *autoFillVisitor) visitBlock(b *hcl.Block) {
	v.visitBody(b.Body)
}

func (v *autoFillVisitor) visitBody(b hcl.Body) {
	bc := bodyContent(b)
	for _, blk := range bc.Blocks {
		v.visitBlock(blk)
	}
	for _, attr := range bc.Attributes {
		v.visitAttribute(attr)
	}
}

// Borrowed from https://github.com/pulumi/pulumi-converter-terraform/blob/master/pkg/convert/tf.go#L1688
func bodyContent(body hcl.Body) *hcl.BodyContent {
	// We want to exclude any hidden blocks and attributes, and the only way to do that with hcl.Body is to
	// give it a schema. JustAttributes() will return all non-hidden attributes, but will error if there's
	// any blocks, and there's no equivalent to get non-hidden attributes and blocks.
	hclSchema := &hcl.BodySchema{}
	// The `body` passed in here _should_ be a hclsyntax.Body. That's currently the only way to just iterate
	// all the raw blocks of a hcl.Body.
	synbody, ok := body.(*hclsyntax.Body)
	contract.Assertf(ok, "%T was not a hclsyntax.Body", body)
	for _, block := range synbody.Blocks {
		if block.Type != "dynamic" {
			hclSchema.Blocks = append(hclSchema.Blocks, hcl.BlockHeaderSchema{Type: block.Type})
		} else {
			// Dynamic blocks have labels on them, we need to tell the schema that's ok.
			hclSchema.Blocks = append(hclSchema.Blocks, hcl.BlockHeaderSchema{
				Type:       block.Type,
				LabelNames: block.Labels,
			})
		}
	}
	for _, attr := range synbody.Attributes {
		hclSchema.Attributes = append(hclSchema.Attributes, hcl.AttributeSchema{Name: attr.Name})
	}
	content, diagnostics := body.Content(hclSchema)
	contract.Assertf(len(diagnostics) == 0, "diagnostics was not empty: %s", diagnostics.Error())
	return content
}
