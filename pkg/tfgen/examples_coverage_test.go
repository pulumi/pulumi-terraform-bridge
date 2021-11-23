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

// This file implements a simple system for locally testing and debugging the
// HCL converter without the use of GitHub Actions.
package tfgen

import (
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/stretchr/testify/assert"
)

func Test_HclConversion(t *testing.T) {

	//============================= HCL code to be given to the converter =============================//
	hcl := `
	resource "aws_iam_role" "iam_for_lambda" {
		name = "iam_for_lambda"
	  
		assume_role_policy = <<EOF
	  {
		"Version": "2012-10-17",
		"Statement": [
		  {
			"Action": "sts:AssumeRole",
			"Principal": {
			  "Service": "lambda.amazonaws.com"
			},
			"Effect": "Allow",
			"Sid": ""
		  }
		]
	  }
	  EOF
	  }
	  
	  resource "aws_lambda_function" "test_lambda" {
		filename      = "lambda_function_payload.zip"
		function_name = "lambda_function_name"
		role          = aws_iam_role.iam_for_lambda.arn
		handler       = "index.test"
	  
		# The filebase64sha256() function is available in Terraform 0.11.12 and later
		# For Terraform 0.11.11 and earlier, use the base64sha256() function and the file() function:
		# source_code_hash = "${base64sha256(file("lambda_function_payload.zip"))}"
		source_code_hash = filebase64sha256("lambda_function_payload.zip")
	  
		runtime = "nodejs12.x"
	  
		environment {
		  variables = {
			foo = "bar"
		  }
		}
	  }
	`
	//=================================================================================================//

	// [go, nodejs, python, dotnet, schema]
	languageName := "go"

	// Creating the Code Generator which will translate our HCL program
	g, err := NewGenerator(GeneratorOptions{
		Version:      "version",
		Language:     Language(languageName),
		Debug:        false,
		SkipDocs:     false,
		SkipExamples: false,
		Sink: diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Auto,
		}),
	})
	assert.NoError(t, err, "Failed to create generator")

	// Attempting to convert our HCL code
	codeBlock, stderr, err := g.convertHCL(hcl, "EXAMPLE_NAME")

	// Checking for error
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(stderr)
	}

	// Printing translated code in the case that it was successfully converted
	fmt.Println(codeBlock)

	// Throwing a panic so that we see the translated code
	panic("")
}
