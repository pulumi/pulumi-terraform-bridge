// Copyright 2016-2023, Pulumi Corporation.
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

package tfbridge

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Defines bridged provider metadata that is pre-computed at build time with tfgen (tfgen
// ("github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfgen") and typically made available to the provider
// binary at runtime with [embed].
//
// [embed]: https://pkg.go.dev/embed
type ProviderMetadata struct {
	// JSON-serialzed Pulumi Package Schema.
	PackageSchema []byte

	// Deprecated: This field is no longer in use and will be removed in future versions.
	BridgeMetadata []byte

	// XParamaterize overrides the functionality of the Paramaterize call.
	//
	// XParamaterize is an experimental API and should not be used by 3rd parties. It
	// does not have a backwards compatibility guarantee and may be removed in the
	// future.
	XParamaterize func(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error)

	// XGetSchema overrides the functionality of the GetSchema call. Either XGetSchema
	// or PackageSchema must be set. If both are set, then the provider will panic
	// during startup.
	//
	// XGetSchema is an experimental API and should not be used by 3rd parties. It
	// does not have a backwards compatibility guarantee and may be removed in the
	// future.
	XGetSchema func(context.Context, plugin.GetSchemaRequest) ([]byte, error)
}
