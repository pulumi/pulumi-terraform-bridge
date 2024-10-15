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

// proto enables building a [shim.Provider] around a [tfprotov6.ProviderServer].
//
// It is intended to help with schema generation, and should not be used for "runtime"
// resource operations like [Provider.Apply], [Provider.Diff], etc.
//
// To view unsupported methods, see ./unsuported.go.
package proto

import (
	pkgpf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto"
)

// TODO(Deprecated): Use github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto.New instead.
var New = pkgpf.New

// TODO(Deprecated): Use github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto.Provider instead.
type Provider = pkgpf.Provider
