// Code copied from github.com/hashicorp/terraform-plugin-go by go generate; DO NOT EDIT.
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package toproto

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"
)

func DynamicValue(in *tfprotov6.DynamicValue) *tfplugin6.DynamicValue {
	if in == nil {
		return nil
	}

	resp := &tfplugin6.DynamicValue{
		Msgpack: in.MsgPack,
		Json:    in.JSON,
	}

	return resp
}

func CtyType(in tftypes.Type) []byte {
	if in == nil {
		return nil
	}

	// MarshalJSON is always error safe.
	// nolint:staticcheck // Intended first-party usage
	resp, _ := in.MarshalJSON()

	return resp
}
