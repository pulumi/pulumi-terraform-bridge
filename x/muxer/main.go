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

package muxer

import (
	"context"
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Mux multiple rpc servers into a single server by routing based on request type and urn.
//
// Most rpc methods are resolved via a schema lookup based precomputed mapping created
// when the Muxer is initialized, with earlier servers getting priority.
//
// For example:
//
//	Given server s1 serving resources r1, r2 and server s2 serving r1, r3, the muxer
//	m1 := Mux(host, s1, s2) will dispatch r1 and r2 to s1. m1 will dispatch only r3 to
//	s2.  If we swap the order of of creation: m2 := Mux(host, s2, s1) we see different
//	prioritization. m2 will serve r1 and r3 to s2, only serving r2 to s1.
//
// Most methods are fully dispatch based:
//
//   - Create, Read, Update, Delete, Check, Diff: The type is extracted from the URN
//     associated with the request. The server who's schema provided the resource is routed
//     the whole request.
//
//   - Construct: The type token is passed directly. The server who's schema provided the
//     resource is routed the whole request.
//
//   - Invoke, StreamInvoke, Call: The type token is passed directly. The server who's
//     schema provided the function is routed the whole request.
//
// Each provider specifies in it's schema what options it accepts as configuration. Config
// based endpoints filter the schema so each provider is only shown keys that it expects
// to see. It is possible for multiple subsidiary providers to accept the same key.
//
//   - CheckConfig: Broadcast to each server. Any diffs between returned results errors.
//
//   - DiffConfig: Broadcast to each server. Results are then merged with the most drastic
//     action dominating.
//
//   - Configure: Broadcast to each server for individual configuration. When computing
//     the returned set of capabilities, each option is set to the AND of the subsidiary
//     servers. This means that the Muxed server is only as capable as the least capable
//     of its subsidiaries.
//
// A dispatch strategy doesn't make sense for methods related to the provider as a
// whole. The following methods are broadcast to all providers:
//
//   - Cancel: Each server receives a cancel request.
//
// The remaining methods are treated specially by the Muxed server:
//
//   - GetSchema: When Mux is called, GetSchema is called once on each server. The muxer
//     merges each schema with earlier servers overriding later servers. The origin of each
//     resource and function in the presented schema is remembered and used to route later
//     resource and function requests.
//
//   - Attach: `Attach` is never called on Muxed providers. Instead the host passed into
//     `Mux` is replaced. If subsidiary servers where constructed with the same `host` as
//     passed to `Mux`, then they will observe the new `host` spurred by `Attach`.
//
//   - GetMapping: `GetMapping` dispatches on all underlerver Servers. If zero or 1 server
//     responds with a non-empty data section, we call GetMappingHandler[Key] to merge the
//     data sections, where Key is the key given in the GetMappingRequest.
type Main struct {
	Servers []Endpoint

	// An optional pre-computed mapping of functions/resources to servers.
	DispatchTable DispatchTable

	// An optional pre-computed schema. If not provided, then the schema will be
	// derived from layering underlying server schemas.
	//
	// If set, DispatchTable must also be set.
	Schema string

	GetMappingHandler map[string]MultiMappingHandler
}

func (m Main) Server(host *provider.HostClient, module, version string) (pulumirpc.ResourceProviderServer, error) {
	servers := make([]rpc.ResourceProviderServer, len(m.Servers))
	for i, s := range m.Servers {
		var err error
		servers[i], err = s.Server(host)
		if err != nil {
			return nil, err
		}
	}

	dispatchTable, pulumiSchema := m.DispatchTable.dispatchTable, m.Schema
	if dispatchTable.isEmpty() || pulumiSchema == "" {
		req := &rpc.GetSchemaRequest{Version: SchemaVersion}
		primary, err := servers[0].GetSchema(context.Background(), req)
		contract.AssertNoErrorf(err, "Muxing requires GetSchema for dispatch")
		targetSchema := new(schema.PackageSpec)
		err = json.Unmarshal([]byte(primary.Schema), targetSchema)
		contract.AssertNoErrorf(err, "primary schema failed to parse")

		schemas := make([]schema.PackageSpec, len(servers))
		for i, s := range servers {
			resp, err := s.GetSchema(context.Background(), req)
			contract.AssertNoErrorf(err, "Server %d failed GetSchema", i)
			o := schema.PackageSpec{}
			err = json.Unmarshal([]byte(resp.GetSchema()), &o)
			contract.AssertNoErrorf(err, "Server %d schema failed to parse", i)
			schemas[i] = o
		}

		mComputed, muxedSchema, err := MergeSchemasAndComputeDispatchTable(schemas)
		contract.AssertNoErrorf(err, "Failed to compute a muxer mapping")
		schemaBytes, err := json.Marshal(muxedSchema)
		contract.AssertNoErrorf(err, "Failed to marshal muxed schema")
		pulumiSchema = string(schemaBytes)
		dispatchTable = mComputed.dispatchTable
	}

	server := mux(host, dispatchTable, pulumiSchema, m.GetMappingHandler, servers...)

	return server, nil
}

type Endpoint struct {
	Server func(*provider.HostClient) (rpc.ResourceProviderServer, error)
}
