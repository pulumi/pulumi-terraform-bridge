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
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

func TestDetectingDanglingRefs(t *testing.T) {
	msgs, err := check("testdata/dangling")
	require.NoError(t, err)

	autogold.Expect([]string{
		"PHL0001: Reference to undeclared resource name=example token=aws_acm_certificate",
		"PHL0001: Reference to undeclared resource name=example token=aws_route53_zone",
	}).Equal(t, msgs)
}

func check(dir string) ([]string, error) {
	c := make(chan issue)
	var failed error
	go func() {
		failed = lint(context.Background(), dir, c)
		close(c)
	}()
	result := []string{}
	for i := range c {
		result = append(result, formatIssue(i))
	}
	sort.Strings(result)
	return result, failed
}

func formatIssue(i issue) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s: %s", i.Code(), i.Detail())
	attrs := []string{}
	m := i.Attributes()
	for k := range m {
		attrs = append(attrs, k)
	}
	sort.Strings(attrs)
	for _, a := range attrs {
		fmt.Fprintf(&buf, " %s=%s", a, m[a])
	}
	return buf.String()
}
