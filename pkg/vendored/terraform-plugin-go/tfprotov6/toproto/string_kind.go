// Code copied from github.com/hashicorp/terraform-plugin-go by go generate; DO NOT EDIT.
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package toproto

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"
)

func StringKind(in tfprotov6.StringKind) tfplugin6.StringKind {
	return tfplugin6.StringKind(in)
}
