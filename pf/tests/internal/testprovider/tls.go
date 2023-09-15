// Copyright 2016-2022, Pulumi Corporation.
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

package testprovider

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"unicode"

	tlsshim "github.com/hashicorp/terraform-provider-tls/shim"
	tfpf "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

//go:embed cmd/pulumi-resource-tls/bridge-metadata.json
var tlsProviderBridgeMetadata []byte

// Adapts tls provider to tfbridge for testing tfbridge against another realistic provider.
func TLSProvider() tfbridge.ProviderInfo {
	tlsPkg := "tls"
	tlsMod := "index"
	tlsVersion := "4.0.4"

	tlsMember := func(mod string, mem string) tokens.ModuleMember {
		return tokens.ModuleMember(tlsPkg + ":" + mod + ":" + mem)
	}

	tlsType := func(mod string, typ string) tokens.Type {
		return tokens.Type(tlsMember(mod, typ))
	}

	tlsDataSource := func(mod string, res string) tokens.ModuleMember {
		fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
		return tlsMember(mod+"/"+fn, res)
	}

	tlsResource := func(mod string, res string) tokens.Type {
		fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
		return tlsType(mod+"/"+fn, res)
	}

	return tfbridge.ProviderInfo{
		Name:             "tls",
		P:                tfpf.ShimProvider(tlsshim.NewProvider()),
		Description:      "A Pulumi package to create TLS resources in Pulumi programs.",
		Keywords:         []string{"pulumi", "tls"},
		License:          "Apache-2.0",
		Homepage:         "https://pulumi.io",
		Repository:       "https://github.com/pulumi/pulumi-tls",
		UpstreamRepoPath: ".",
		Resources: map[string]*tfbridge.ResourceInfo{
			"tls_cert_request":        {Tok: tlsResource(tlsMod, "CertRequest")},
			"tls_locally_signed_cert": {Tok: tlsResource(tlsMod, "LocallySignedCert")},
			"tls_private_key":         {Tok: tlsResource(tlsMod, "PrivateKey")},
			"tls_self_signed_cert":    {Tok: tlsResource(tlsMod, "SelfSignedCert")},
		},
		DataSources: map[string]*tfbridge.DataSourceInfo{
			"tls_public_key":  {Tok: tlsDataSource(tlsMod, "getPublicKey")},
			"tls_certificate": {Tok: tlsDataSource(tlsMod, "getCertificate")},
		},
		JavaScript: &tfbridge.JavaScriptInfo{
			Dependencies: map[string]string{
				"@pulumi/pulumi": "^3.0.0",
			},
			DevDependencies: map[string]string{
				"@types/node": "^10.0.0", // so we can access strongly typed node definitions.
			},
		},
		Python: &tfbridge.PythonInfo{
			Requires: map[string]string{
				"pulumi": ">=3.0.0,<4.0.0",
			},
		},
		Golang: &tfbridge.GolangInfo{
			ImportBasePath: filepath.Join(
				fmt.Sprintf("github.com/pulumi/pulumi-%[1]s/sdk/", tlsPkg),
				tfbridge.GetModuleMajorVersion(tlsVersion),
				"go",
				tlsPkg,
			),
			GenerateResourceContainerTypes: true,
		},
		CSharp: &tfbridge.CSharpInfo{
			PackageReferences: map[string]string{
				"Pulumi": "3.*",
			},
			Namespaces: map[string]string{
				"tls": "Tls",
			},
		},
		Version:      tlsVersion,
		MetadataInfo: tfbridge.NewProviderMetadata(tlsProviderBridgeMetadata),
	}
}
