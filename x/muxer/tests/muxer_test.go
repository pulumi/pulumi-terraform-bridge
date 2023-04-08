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

package muxer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"

	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
)

func TestSimpleDispatch(t *testing.T) {
	var m muxer.ComputedMapping
	m.Resources = map[string]int{
		"test:mod:A": 0,
		"test:mod:B": 1,
	}

	replayMux(t, m,
		simpleExchange(0, "/pulumirpc.ResourceProvider/Create", `{
            "urn": "urn:pulumi:test-stack::basicprogram::test:mod:A::r1",
            "properties": {
              "ecdsacurve": "P384"
            },
            "preview": true
          }`, `{
            "id": "r1",
            "properties": {
              "id": "rA"
            }
          }`),
		simpleExchange(1, "/pulumirpc.ResourceProvider/Create", `{
            "urn": "urn:pulumi:test-stack::basicprogram::test:mod:B::r1",
            "properties": {
              "ecdsacurve": "P384"
            }
          }`, `{
            "id": "r1",
            "properties": {
              "id": "rB"
            }
          }`),
	)
}

func TestConfigure(t *testing.T) {
	var m muxer.ComputedMapping
	m.Config = map[string][]int{
		"a": []int{0},
		"b": []int{0, 1},
		"c": []int{1},
	}

	replayMux(t, m,
		exchange("/pulumirpc.ResourceProvider/Configure", `{
      "args": {
        "a": "1",
        "b": "2",
        "c": "3"
      }
    }`, `{
      "supportsPreview": true
  }`,
			part(0, `{
  "args": {
    "a": "1",
    "b": "2"
  }
}`, `{
  "acceptSecrets": true,
  "supportsPreview": true
}`),
			part(1, `{
  "args": {
    "b": "2",
    "c": "3"
  }
}`, `{
  "supportsPreview": true,
  "acceptResources": true
}`),
		))
}

func replayMux(t *testing.T, mapping muxer.ComputedMapping, exchanges ...Exchange) {
	serverBehavior := [][]call{}
	for _, ex := range exchanges {
		for _, part := range ex.Parts {
			for part.Provider >= len(serverBehavior) {
				serverBehavior = append(serverBehavior, nil)
			}
			serverBehavior[part.Provider] = append(serverBehavior[part.Provider],
				call{
					incoming: string(part.Request),
					response: string(part.Response),
				})
		}
	}
	servers := make([]rpc.ResourceProviderServer, len(serverBehavior))
	for i, s := range serverBehavior {
		servers[i] = &server{t: t, calls: s}
	}
	muxedServer := buildMux(t, mapping, servers...)

	bytes, err := json.Marshal(exchanges)
	require.NoError(t, err)
	testutils.ReplaySequence(t, muxedServer, string(bytes))
}

type Exchange struct {
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Parts    []ExchangePart  `json:"-"`
}

type ExchangePart struct {
	Provider int
	Request  string `json:"request"`
	Response string `json:"response"`
}

// A simple exchange is one where only one sub-server is used
func simpleExchange(provider int, method, request, response string) Exchange {
	return Exchange{
		Method:   method,
		Request:  json.RawMessage(request),
		Response: json.RawMessage(response),
		Parts: []ExchangePart{
			{
				Provider: provider,
				Request:  request,
				Response: response,
			},
		},
	}
}

func exchange(method, request, response string, parts ...ExchangePart) Exchange {
	return Exchange{
		Method:   method,
		Request:  json.RawMessage(request),
		Response: json.RawMessage(response),
		Parts:    parts,
	}
}

func part(provider int, request, response string) ExchangePart {
	return ExchangePart{
		provider,
		request,
		response,
	}
}

func buildMux(
	t *testing.T, mapping muxer.ComputedMapping, servers ...rpc.ResourceProviderServer,
) rpc.ResourceProviderServer {
	endpoints := make([]muxer.Endpoint, len(servers))
	for i, s := range servers {
		i, s := i, s
		endpoints[i] = muxer.Endpoint{
			Server: func(*provider.HostClient) (rpc.ResourceProviderServer, error) {
				return s, nil
			},
		}

	}
	s, err := muxer.Main{
		Servers:         endpoints,
		ComputedMapping: mapping,
		Schema:          "some-schema",
	}.Server(nil, "test", "0.0.0")
	require.NoError(t, err)
	return s
}

var _ rpc.ResourceProviderServer = ((*server)(nil))

type server struct {
	rpc.UnimplementedResourceProviderServer

	t *testing.T

	calls []call
}

type call struct {
	incoming string
	response string
}

func handleCall[T proto.Message, R proto.Message](m *server, req T) (R, error) {
	next := m.calls[0]
	m.calls = m.calls[1:]

	// This is actually a *T where *T implements proto.Message. To create the
	// settable value, we need to hydrate the underlying pointer.
	var r R
	reflect.ValueOf(&r).Elem().Set(reflect.New(reflect.TypeOf(r).Elem()))

	marshalled, err := protojson.MarshalOptions{Multiline: true}.Marshal(req)
	require.NoError(m.t, err)

	failed := m.t.Failed()
	testutils.AssertJSONMatchesPattern(m.t, json.RawMessage(next.incoming), json.RawMessage(marshalled))
	if !failed && m.t.Failed() {
		m.t.Logf("Unexpected semantic diff:\nexpected: <-JSON\n%s\nJSON\nactual: <-JSON\n%s\nJSON\n",
			next.incoming, string(marshalled))
	}
	err = protojson.Unmarshal([]byte(next.response), r)
	return r, err
}

func (m *server) GetSchema(ctx context.Context, req *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	return handleCall[*rpc.GetSchemaRequest, *rpc.GetSchemaResponse](m, req)
}

func (m *server) CheckConfig(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return handleCall[*rpc.CheckRequest, *rpc.CheckResponse](m, req)
}

func (m *server) DiffConfig(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return handleCall[*rpc.DiffRequest, *rpc.DiffResponse](m, req)
}

func (m *server) Configure(ctx context.Context, req *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	return handleCall[*rpc.ConfigureRequest, *rpc.ConfigureResponse](m, req)
}

func (m *server) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	return handleCall[*rpc.InvokeRequest, *rpc.InvokeResponse](m, req)
}

func (m *server) StreamInvoke(req *rpc.InvokeRequest, s rpc.ResourceProvider_StreamInvokeServer) error {
	assert.Fail(m.t, "StreamInvoke not implemented on `server`")
	return fmt.Errorf("UNIMPLEMENTED")
}

func (m *server) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	return handleCall[*rpc.CallRequest, *rpc.CallResponse](m, req)
}

func (m *server) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return handleCall[*rpc.CheckRequest, *rpc.CheckResponse](m, req)
}

func (m *server) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return handleCall[*rpc.DiffRequest, *rpc.DiffResponse](m, req)
}

func (m *server) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	return handleCall[*rpc.CreateRequest, *rpc.CreateResponse](m, req)
}

func (m *server) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return handleCall[*rpc.ReadRequest, *rpc.ReadResponse](m, req)
}

func (m *server) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	return handleCall[*rpc.UpdateRequest, *rpc.UpdateResponse](m, req)
}

func (m *server) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return handleCall[*rpc.DeleteRequest, *emptypb.Empty](m, req)
}

func (m *server) Construct(ctx context.Context, req *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
	return handleCall[*rpc.ConstructRequest, *rpc.ConstructResponse](m, req)
}

func (m *server) Cancel(ctx context.Context, e *emptypb.Empty) (*emptypb.Empty, error) {
	return handleCall[*emptypb.Empty, *emptypb.Empty](m, e)
}

func (m *server) GetPluginInfo(ctx context.Context, e *emptypb.Empty) (*rpc.PluginInfo, error) {
	return handleCall[*emptypb.Empty, *rpc.PluginInfo](m, e)
}

func (m *server) Attach(ctx context.Context, req *rpc.PluginAttach) (*emptypb.Empty, error) {
	return handleCall[*rpc.PluginAttach, *emptypb.Empty](m, req)
}

func (m *server) GetMapping(ctx context.Context, req *rpc.GetMappingRequest) (*rpc.GetMappingResponse, error) {
	return handleCall[*rpc.GetMappingRequest, *rpc.GetMappingResponse](m, req)
}
