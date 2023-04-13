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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkBridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
)

// Implements main() or a bridged Pulumi plugin, complete with argument parsing.
func Main(ctx context.Context, pkg string, prov ProviderInfo, meta ProviderMetadata) {
	handleFlags(ctx, prov, meta, prov.Version)
	// TODO[pulumi/pulumi-terraform-bridge#820]
	// prov.P.InitLogging()

	if err := serve(ctx, pkg, prov, meta); err != nil {
		cmdutil.ExitError(err.Error())
	}
}

func handleFlags(ctx context.Context, prov ProviderInfo, meta ProviderMetadata, version string) {
	// Look for a request to dump the provider info to stdout.
	flags := flag.NewFlagSet("tf-provider-flags", flag.ContinueOnError)

	// Discard print output by default; there might be flags such
	// as -tracing that are unrecognized at this phase but will be
	// parsed later by `Serve`. We do not want to print errors
	// about them. Save `defaultOutput` for help below.
	defaultOutput := flags.Output()
	flags.SetOutput(io.Discard)

	dumpInfo := flags.Bool("get-provider-info", false, "dump provider info as JSON to stdout")
	providerVersion := flags.Bool("version", false, "get built provider version")

	err := flags.Parse(os.Args[1:])
	contract.IgnoreError(err)

	// Ensure we do print help message when `--help` is requested.
	if err == flag.ErrHelp {
		flags.SetOutput(defaultOutput)
		err := flags.Parse(os.Args[1:])
		if err != nil {
			cmdutil.ExitError(err.Error())
		}
	}

	if *dumpInfo {
		pp, err := newProviderWithContext(ctx, prov, meta)
		if err != nil {
			cmdutil.ExitError(err.Error())
		}
		info := pp.(*provider).marshalProviderInfo(ctx)
		if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
			cmdutil.ExitError(err.Error())
		}
		os.Exit(0)
	}

	if *providerVersion {
		fmt.Println(version)
		os.Exit(0)
	}
}

// Implements main() or a bridged Pulumi plugin, complete with argument parsing.
//
// This is an experimental API.
func MainWithMuxer(ctx context.Context, pkg string, meta ProviderMetadata, infos ...Muxed) {
	version := infos[0].GetInfo().Version
	schema := string(meta.PackageSchema)
	mapping, found, err := metadata.Get[muxer.ComputedMapping](infos[0].GetInfo().GetMetadata(), "mux")
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
	if !found {
		fmt.Printf("Missing precomputed mapping. Did you run `make tfgen`?")
		os.Exit(1)
	}
	m := muxer.Main{
		ComputedMapping: mapping,
		Schema:          schema,
		GetMappingHandler: map[string]muxer.MultiMappingHandler{
			"tf":        combineTFGetMappingKey,
			"terraform": combineTFGetMappingKey,
		},
	}

	err = rprovider.Main(pkg, func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		for _, info := range infos {
			info := info // https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables

			// Add PF based servers to the runtime.
			if info.PF != nil {
				m.Servers = append(m.Servers, muxer.Endpoint{
					Server: func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {
						return newProviderServer(ctx, host, *info.PF, meta)
					}})
				continue
			}
			m.Servers = append(m.Servers, muxer.Endpoint{
				Server: func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {
					return tfbridge.NewProvider(ctx, host, pkg, version, info.SDK.P, *info.SDK, meta.PackageSchema), nil
				}})
		}
		return m.Server(host, pkg, version)
	})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}

func combineTFGetMappingKey(_ string, infos [][]byte) ([]byte, error) {
	var target *tfbridge.MarshallableProviderInfo
	for i, info := range infos {
		var src tfbridge.MarshallableProviderInfo
		err := json.Unmarshal(info, &src)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal info %d: %w", i, err)
		}
		if i == 0 {
			target = &src
			continue
		}

		mergeProviderInfo(target, src)
	}
	return json.Marshal(target)
}

// Merge two tfbridge.MarshallableProviderInfo structs into one-another.
//
// Fields from src are merged into dst, prioritizing data in dst.
//
// The fields Name, Version and TFProviderVersion are not merged.
func mergeProviderInfo(dst *tfbridge.MarshallableProviderInfo, src tfbridge.MarshallableProviderInfo) {
	contract.Assertf(dst != nil, "cannot copy into nil pointer")
	mergeUnder(&dst.Provider, src.Provider, mergeProvider)
	mergeMapUnder(&dst.Config, src.Config)
	mergeMapUnder(&dst.Resources, src.Resources)
	mergeMapUnder(&dst.DataSources, src.DataSources)
}

// Merge two tfbridge.MarshallableProvider structs into one-another.
//
// Fields from src are merged into dst, prioritizing data from dst.
func mergeProvider(dst *tfbridge.MarshallableProvider, src tfbridge.MarshallableProvider) {
	mergeMapUnder(&dst.Schema, src.Schema)
	mergeMapUnder(&dst.Resources, src.Resources)
	mergeMapUnder(&dst.DataSources, src.DataSources)
}

// A helper function to merge two structs together, accounting for the structs being nil.
func mergeUnder[T any](dst **T, src *T, merge func(*T, T)) {
	if src == nil {
		return
	}
	if *dst == nil {
		*dst = src
	}
	merge(*dst, *src)
}

// A helper function to merge two maps together, accounting for the maps being nil.
func mergeMapUnder[K comparable, V any](dst *map[K]V, src map[K]V) {
	if src == nil {
		return
	}
	if *dst == nil {
		*dst = src
	}
	for k, v := range src {
		_, skip := (*dst)[k]
		if !skip {
			(*dst)[k] = v
		}
	}
}

// A union of pf and sdk based ProviderInfo for use in MainWithMuxer.
// Exactly 1 field of this struct should hold a value
//
// This is an experimental API.
type Muxed struct {
	PF  *ProviderInfo
	SDK *sdkBridge.ProviderInfo
}

func (m Muxed) GetInfo() sdkBridge.ProviderInfo {
	if m.PF == nil {
		return *m.SDK
	}
	return m.PF.ProviderInfo
}
