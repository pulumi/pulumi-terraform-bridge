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
	"errors"
	"reflect"
	"testing"

	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/muxer"
)

func TestSimpleDispatch(t *testing.T) {
	t.Parallel()
	var m muxer.DispatchTable
	m.Resources = map[string]int{
		"test:mod:A": 0,
		"test:mod:B": 1,
	}

	mux(t, m).replay(
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

func TestCheckConfigErrorNotDuplicated(t *testing.T) {
	t.Parallel()
	var m muxer.DispatchTable
	m.Resources = map[string]int{
		"test:mod:A": 0,
		"test:mod:B": 1,
	}
	errString := []string{"myerr"}
	mux(t, m).replay(
		exchange("/pulumirpc.ResourceProvider/CheckConfig", "{}", "{}", errString,
			part(0, "{}", "{}", errString),
			part(1, "{}", "{}", errString),
		))
}

func TestCheckConfigDifferentErrorsNotDropped(t *testing.T) {
	t.Parallel()
	var m muxer.DispatchTable
	m.Resources = map[string]int{
		"test:mod:A": 0,
		"test:mod:B": 1,
	}
	firstErr := []string{"myerr"}
	secondErr := []string{"othererr"}
	expectedErr := []string{"2 errors occurred:\n\t* myerr\n\t* othererr\n\n"}
	mux(t, m).replay(
		exchange(
			"/pulumirpc.ResourceProvider/CheckConfig",
			"{}",
			"{}",
			expectedErr,
			part(0, "{}", "{}", firstErr),
			part(1, "{}", "{}", secondErr),
		))
}

func TestCheckConfigOneErrorReturned(t *testing.T) {
	t.Parallel()
	var m muxer.DispatchTable
	m.Resources = map[string]int{
		"test:mod:A": 0,
		"test:mod:B": 1,
	}
	err := []string{"myerr"}
	mux(t, m).replay(
		exchange("/pulumirpc.ResourceProvider/CheckConfig", "{}", "{}", err,
			part(0, "{}", `{"inputs": {"myurn":"urn"}}`, nil),
			part(1, "{}", "{}", err),
		))
}

func TestMuxerConfigure(t *testing.T) {
	t.Parallel()
	var m muxer.DispatchTable
	m.Resources = map[string]int{
		"test:mod:A": 0,
		"test:mod:B": 1,
	}

	mux(t, m).replay(
		exchange("/pulumirpc.ResourceProvider/Configure", `{
      "args": {
        "a": "1",
        "b": "2",
        "c": "3"
      }
    }`, `{
      "supportsPreview": true,
      "supportsAutonamingConfiguration": true
  }`, nil,
			part(0, `{
  "args": {
    "a": "1",
    "b": "2",
    "c": "3"
  }
}`, `{
  "acceptSecrets": true,
  "supportsPreview": true,
  "supportsAutonamingConfiguration": true
}`, nil),
			part(1, `{
  "args": {
    "a": "1",
    "b": "2",
    "c": "3"
  }
}`, `{
  "supportsPreview": true,
  "acceptResources": true,
  "supportsAutonamingConfiguration": true
}`, nil),
		))
}

func TestDivergentCheckConfig(t *testing.T) {
	t.Parallel()
	// Early versions of muxer failed hard on divergent responses from CheckConfig. This test ensures that it can
	// tolerate such responses (with logging or warning). The practical case is divergent handling of secret markers
	// where pf and v3 based providers respond with the same value but do not agree on the secret markers.
	req := `
	{
	  "urn": "urn:pulumi:repro::label-gitlab::pulumi:providers:gitlab::default",
	  "olds": {},
	  "news": {
	    "token": "verysecrettoken",
	    "version": "5.0.1"
	  }
	}`
	resp0 := `
	{
	  "inputs": {
	    "token": {
	      "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
	      "value": "verysecrettoken"
	    },
	    "version": "5.0.1"
	  }
	}`
	resp1 := `
        {
	  "inputs": {
	    "token": "verysecrettoken",
	    "version": "5.0.1"
	  }
	}`
	muxedResp := resp0
	e := exchange("/pulumirpc.ResourceProvider/CheckConfig", req, muxedResp, nil,
		part(0, req, resp0, nil),
		part(1, req, resp1, nil))

	m := muxer.DispatchTable{}
	m.Resources = map[string]int{}
	mux(t, m).replay(e)
}

func TestMuxerGetMapping(t *testing.T) {
	t.Parallel()
	t.Run("single-responding-server", func(t *testing.T) {
		var m muxer.DispatchTable
		m.Resources = map[string]int{
			"test:mod:A": 0,
			"test:mod:B": 1,
		}

		mux(t, m).replay(
			exchange("/pulumirpc.ResourceProvider/GetMapping", `{
  "key": "k1"
}`, `{
  "provider": "p1",
  "data": "dw=="`+ /* the base64 encoding of d1 */ `
}`, nil, part(0, `{
  "key": "k1"
}`, `{
  "provider": "p1",
  "data": "d1"
}`, nil), part(1, `{
  "key": "k1"
}`, `{
  "provider": "",
  "data": ""
}`, nil)))
	})
	t.Run("merged-responding-server", func(t *testing.T) {
		var m muxer.DispatchTable
		m.Resources = map[string]int{
			"test:mod:A": 0,
			"test:mod:B": 1,
		}

		combine := func(args muxer.GetMappingArgs) (muxer.GetMappingResponse, error) {
			result := args.Fetch()
			assert.Equal(t, "p1", result[0].Provider)
			assert.Len(t, result, 2)
			assert.Equalf(t, "d1", string(result[0].Data), "first sub-server")
			assert.Equalf(t, "d2", string(result[1].Data), "second sub-server")
			return muxer.GetMappingResponse{
				Provider: "p1",
				Data:     []byte("r1"),
			}, nil
		}

		mux(t, m).getMappingHandler("k", combine).replay(
			exchange("/pulumirpc.ResourceProvider/GetMapping", `{
  "key": "k"
}`, `{
  "provider": "p1",
  "data": "cjE="`+ /* the base64 encoding of r1 */ `
}`, nil, part(0, `{
  "key": "k"
}`, `{
  "provider": "p1",
  "data": "ZDE="`+ /* the base64 encoding of d1*/ `
}`, nil), part(1, `{
  "key": "k"
}`, `{
  "provider": "p1",
  "data": "ZDI="`+ /* the base64 encoding of d2*/ `
}`, nil)))
	})
}

type testMuxer struct {
	t                  *testing.T
	mapping            muxer.DispatchTable
	getMappingHandlers map[string]muxer.MultiMappingHandler
}

func mux(t *testing.T, mapping muxer.DispatchTable) testMuxer {
	return testMuxer{t, mapping, nil}
}

func (m testMuxer) getMappingHandler(key string, f muxer.MultiMappingHandler) testMuxer {
	if m.getMappingHandlers == nil {
		m.getMappingHandlers = map[string]muxer.MultiMappingHandler{}
	}
	m.getMappingHandlers[key] = f
	return m
}

func (m testMuxer) replay(exchanges ...Exchange) {
	serverBehavior := [][]call{}
	for _, ex := range exchanges {
		for _, part := range ex.Parts {
			for part.Provider >= len(serverBehavior) {
				serverBehavior = append(serverBehavior, nil)
			}
			serverBehavior[part.Provider] = append(serverBehavior[part.Provider],
				call{
					incoming: part.Request,
					response: part.Response,
					errors:   part.Errors,
				})
		}
	}
	servers := make([]pulumirpc.ResourceProviderServer, len(serverBehavior))
	for i, s := range serverBehavior {
		servers[i] = &server{t: m.t, calls: s}
	}
	muxedServer := buildMux(m.t, m.mapping, m.getMappingHandlers, servers...)

	bytes, err := json.Marshal(exchanges)
	require.NoError(m.t, err)
	testutils.ReplaySequence(m.t, muxedServer, string(bytes))
}

type Exchange struct {
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Errors   []string
	Parts    []ExchangePart `json:"-"`
}

type ExchangePart struct {
	Provider int
	Request  string `json:"request"`
	Response string `json:"response"`
	Errors   []string
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

func exchange(method, request, response string, errors []string, parts ...ExchangePart) Exchange {
	return Exchange{
		Method:   method,
		Request:  json.RawMessage(request),
		Response: json.RawMessage(response),
		Errors:   errors,
		Parts:    parts,
	}
}

func part(provider int, request, response string, errors []string) ExchangePart {
	return ExchangePart{
		provider,
		request,
		response,
		errors,
	}
}

func buildMux(
	t *testing.T, mapping muxer.DispatchTable,
	getMappings map[string]muxer.MultiMappingHandler,
	servers ...pulumirpc.ResourceProviderServer,
) pulumirpc.ResourceProviderServer {
	endpoints := make([]muxer.Endpoint, len(servers))
	for i, s := range servers {
		i, s := i, s
		endpoints[i] = muxer.Endpoint{
			Server: func(*provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
				return s, nil
			},
		}
	}
	s, err := muxer.Main{
		Servers:           endpoints,
		DispatchTable:     mapping,
		Schema:            []byte("some-schema"),
		GetMappingHandler: getMappings,
	}.Server(nil, "test", "0.0.0")
	require.NoError(t, err)
	return s
}

var _ pulumirpc.ResourceProviderServer = ((*server)(nil))

type server struct {
	pulumirpc.UnimplementedResourceProviderServer

	t *testing.T

	calls []call
}

type call struct {
	incoming string
	response string
	errors   []string
}

// Assert that a gRPC call matches the next expected call, then rehydrate and return the
// next scheduled response.
func handleMethod[T proto.Message, R proto.Message](m *server, req T) (R, error) {
	next := m.calls[0]
	m.calls = m.calls[1:]

	// R is actually a *T where *T implements proto.Message. To create the settable
	// value, we need to hydrate the underlying pointer.
	var r R
	reflect.ValueOf(&r).Elem().Set(reflect.New(reflect.TypeOf(r).Elem()))

	if next.errors != nil {
		return r, errors.New(next.errors[0])
	}

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

func (m *server) GetSchema(ctx context.Context, req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
	return handleMethod[*pulumirpc.GetSchemaRequest, *pulumirpc.GetSchemaResponse](m, req)
}

func (m *server) CheckConfig(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return handleMethod[*pulumirpc.CheckRequest, *pulumirpc.CheckResponse](m, req)
}

func (m *server) DiffConfig(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return handleMethod[*pulumirpc.DiffRequest, *pulumirpc.DiffResponse](m, req)
}

func (m *server) Configure(ctx context.Context, req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	return handleMethod[*pulumirpc.ConfigureRequest, *pulumirpc.ConfigureResponse](m, req)
}

func (m *server) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	return handleMethod[*pulumirpc.InvokeRequest, *pulumirpc.InvokeResponse](m, req)
}

func (m *server) Call(ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	return handleMethod[*pulumirpc.CallRequest, *pulumirpc.CallResponse](m, req)
}

func (m *server) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return handleMethod[*pulumirpc.CheckRequest, *pulumirpc.CheckResponse](m, req)
}

func (m *server) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return handleMethod[*pulumirpc.DiffRequest, *pulumirpc.DiffResponse](m, req)
}

func (m *server) Create(ctx context.Context, req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	return handleMethod[*pulumirpc.CreateRequest, *pulumirpc.CreateResponse](m, req)
}

func (m *server) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	return handleMethod[*pulumirpc.ReadRequest, *pulumirpc.ReadResponse](m, req)
}

func (m *server) Update(ctx context.Context, req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	return handleMethod[*pulumirpc.UpdateRequest, *pulumirpc.UpdateResponse](m, req)
}

func (m *server) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	return handleMethod[*pulumirpc.DeleteRequest, *emptypb.Empty](m, req)
}

func (m *server) Construct(ctx context.Context, req *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	return handleMethod[*pulumirpc.ConstructRequest, *pulumirpc.ConstructResponse](m, req)
}

func (m *server) Cancel(ctx context.Context, e *emptypb.Empty) (*emptypb.Empty, error) {
	return handleMethod[*emptypb.Empty, *emptypb.Empty](m, e)
}

func (m *server) GetPluginInfo(ctx context.Context, e *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return handleMethod[*emptypb.Empty, *pulumirpc.PluginInfo](m, e)
}

func (m *server) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return handleMethod[*pulumirpc.PluginAttach, *emptypb.Empty](m, req)
}

func (m *server) GetMapping(
	ctx context.Context, req *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	return handleMethod[*pulumirpc.GetMappingRequest, *pulumirpc.GetMappingResponse](m, req)
}
