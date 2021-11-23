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

// This file implements a simple system for locally testing and efficiently
// debugging the HCL converter without the use of GitHub Actions.
package tfgen

import (
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
)

func Test_HclConversion(t *testing.T) {

	//============================= HCL code to be given to the converter =============================//
	hcl := `
	resource "aws_vpc" "vpc" {
		cidr_block = "10.0.0.0/16"
	  }
	  
	  resource "aws_vpn_gateway" "vpn_gateway" {
		vpc_id = aws_vpc.vpc.id
	  }
	  
	  resource "aws_customer_gateway" "customer_gateway" {
		bgp_asn    = 65000
		ip_address = "172.0.0.1"
		type       = "ipsec.1"
	  }
	  
	  resource "aws_vpn_connection" "main" {
		vpn_gateway_id      = aws_vpn_gateway.vpn_gateway.id
		customer_gateway_id = aws_customer_gateway.customer_gateway.id
		type                = "ipsec.1"
		static_routes_only  = true
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
			Color: colors.Never,
		}),
	})
	assert.NoError(t, err, "Failed to create generator")

	// Attempting to convert our HCL code
	codeBlock, stderr, err := g.convertHCL(hcl, "EXAMPLE_NAME")

	// Checking for error and printing if it exists
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(stderr)
	}

	// Printing translated code in the case that it was successfully converted
	fmt.Println(codeBlock)

	assert.NoError(t, err)
}
