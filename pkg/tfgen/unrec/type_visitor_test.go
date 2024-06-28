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

package unrec

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"
)

func TestTypeVisitor(t *testing.T) {
	s := exampleSchema(t)

	starterTypes := []tokens.Type{}
	for _, r := range s.Resources {
		for _, p := range r.Properties {
			if ref, ok := parseLocalRef(p.TypeSpec.Ref); ok {
				starterTypes = append(starterTypes, ref)
			}
		}
	}

	count := 0

	visited := map[tokens.Type][]tokens.Type{}

	vis := &typeVisitor{Schema: s, Visit: func(ancestors []tokens.Type, current tokens.Type) bool {
		visited[current] = ancestors
		count++
		return true
	}}
	vis.VisitTypes(starterTypes...)

	require.Equal(t, 90, count)

	tok := "myprov:index/WebAclStatementRateBasedStatementScopeDownStatementAndStatementStatementAndStatementStatement:WebAclStatementRateBasedStatementScopeDownStatementAndStatementStatementAndStatementStatement"
	require.Equal(t, []tokens.Type{
		"myprov:index/WebAclStatement:WebAclStatement",
		"myprov:index/WebAclStatementRateBasedStatement:WebAclStatementRateBasedStatement",
		"myprov:index/WebAclStatementRateBasedStatementScopeDownStatement:WebAclStatementRateBasedStatementScopeDownStatement",
		"myprov:index/WebAclStatementRateBasedStatementScopeDownStatementAndStatement:WebAclStatementRateBasedStatementScopeDownStatementAndStatement",
		"myprov:index/WebAclStatementRateBasedStatementScopeDownStatementAndStatementStatement:WebAclStatementRateBasedStatementScopeDownStatementAndStatementStatement",
		"myprov:index/WebAclStatementRateBasedStatementScopeDownStatementAndStatementStatementAndStatement:WebAclStatementRateBasedStatementScopeDownStatementAndStatementStatementAndStatement",
	}, visited[tokens.Type(tok)])
}
