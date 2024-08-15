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
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/terraform/pkg/configs"
)

func lintDanglingRefs(ctx context.Context, mod *configs.Module, sink chan<- issue) {
	vis := newDanglingRefVisitor()
	for _, mr := range mod.ManagedResources {
		vis.visitManagedResource(mr)
	}
	for _, r := range vis.dangling() {
		if r.Name() == "value" && r.Token() == "each" {
			continue
		}
		sink <- &danglingRefIssue{
			name:  r.Name(),
			token: r.Token(),
		}
	}
}

type danglingRefIssue struct {
	token string
	name  string
}

func (d *danglingRefIssue) Code() string {
	return "PHL0001"
}

func (d *danglingRefIssue) Detail() string {
	return "Reference to undeclared resource"
}

func (d *danglingRefIssue) Attributes() map[string]string {
	return map[string]string{
		"token": d.token,
		"name":  d.name,
	}
}

var _ issue = (*danglingRefIssue)(nil)

type hclRef string

func (x hclRef) Token() string {
	return strings.Split(string(x), ":::")[0]
}

func (x hclRef) Name() string {
	return strings.Split(string(x), ":::")[1]
}

func newHclRef(token, name string) hclRef {
	return hclRef(fmt.Sprintf("%s:::%s", token, name))
}

type danglingRefVisitor struct {
	defined    map[hclRef]struct{}
	referenced map[hclRef]struct{}
}

func newDanglingRefVisitor() *danglingRefVisitor {
	return &danglingRefVisitor{
		defined:    map[hclRef]struct{}{},
		referenced: map[hclRef]struct{}{},
	}
}

func (v *danglingRefVisitor) dangling() []hclRef {
	d := []hclRef{}
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

func (v *danglingRefVisitor) visitManagedResource(res *configs.Resource) {
	v.defined[newHclRef(res.Type, res.Name)] = struct{}{}
	v.visitBody(res.Config)
	v.visitExpr(res.Count)
	v.visitExpr(res.ForEach)
	v.visitTraversals(res.DependsOn)
	v.visitExprs(res.TriggersReplacement)
}

func (v *danglingRefVisitor) visitTraversal(t hcl.Traversal) {
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
	v.referenced[newHclRef(root.Name, attr.Name)] = struct{}{}
}

func (v *danglingRefVisitor) visitTraversals(ts []hcl.Traversal) {
	for _, t := range ts {
		v.visitTraversal(t)
	}
}

func (v *danglingRefVisitor) visitAttribute(a *hcl.Attribute) {
	v.visitExpr(a.Expr)
}

func (v *danglingRefVisitor) visitExpr(expr hcl.Expression) {
	if expr == nil {
		return
	}
	for _, t := range expr.Variables() {
		v.visitTraversal(t)
	}
}

func (v *danglingRefVisitor) visitExprs(exprs []hcl.Expression) {
	for _, e := range exprs {
		v.visitExpr(e)
	}
}

func (v *danglingRefVisitor) visitBlock(b *hcl.Block) {
	v.visitBody(b.Body)
}

func (v *danglingRefVisitor) visitBody(b hcl.Body) {
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
