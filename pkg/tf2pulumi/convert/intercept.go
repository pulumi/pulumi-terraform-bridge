// Copyright 2016-2021, Pulumi Corporation.
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

package convert

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
)

const logFile = "/Users/anton/pulumi-7949/intercept.log"

// With top-level panic: in Convert  warning: 1628/1628 documentation code blocks failed to convert
// With panic in interceptConversion:         1079/1628 doc code blocks failed to convert
// With mainline:                               51/1628 documentation code blocks failed to convert

func interceptConversion(
	options Options,
	program *pcl.Program,
	generatedFiles map[string][]byte) {

	interceptLog("\n\n## begin %s\n", options.JobDescription)
	defer interceptLog("\n## end %s\n\n", options.JobDescription)

	interceptLog("dumping HCL:\n%s\n", options.SourceHCL)

	interceptLog("program: \n")

	for i, node := range program.Nodes {
		res, isRes := node.(*pcl.Resource)
		if isRes {
			interceptLog("Node %d: %v\n", i, res.Definition)
		} else {
			locVar, isLocVar := node.(*pcl.LocalVariable)
			if isLocVar {
				interceptLog("Node %d: %v\n", i, locVar.Definition)
			} else {
				interceptLog("Node %d: %v\n", i, reflect.TypeOf(node))
			}
		}

	}

	for k, v := range generatedFiles {
		interceptLog("generated file %s:\n%s\n\n", k, string(v))
	}

	funCalls := map[string]int{}

	for _, node := range program.Nodes {

		var pre model.ExpressionVisitor = func(expr model.Expression) (model.Expression, hcl.Diagnostics) {

			funCall, isFunCall := expr.(*model.FunctionCallExpression)

			if isFunCall {
				funCalls[funCall.Syntax.Name]++

				interceptLog("Funcall as part of %s: %v\n",
					options.JobDescription,
					funCall)

				interceptLog("funCall.Name = %s\n", funCall.Name)
				interceptLog("funCall.Tokens.GetName(funCall.Name) = %s\n",
					funCall.Tokens.GetName(funCall.Name))

				for i, arg := range funCall.Args {
					interceptLog("arg at position %d is %v\n", i, arg)
				}

			}

			return expr, nil
		}

		node.VisitExpressions(pre, nil)
	}

	var summary []string
	for f, n := range funCalls {
		summary = append(summary, fmt.Sprintf("%s: %d", f, n))
	}

	if len(summary) > 0 {
		interceptLog("function calls: %s\n",
			strings.Join(summary, ", "))
	} else {
		interceptLog("NO function calls\n")

	}
}

func interceptLog(format string, args ...interface{}) {
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Sprintf("Failed to open logfile: %v", err))
	}
	defer f.Close()
	fmt.Fprintf(f, format, args...)
}
