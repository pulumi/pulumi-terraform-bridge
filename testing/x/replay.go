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

package testing

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	jsonpb "google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type ReplayOptions struct {
	MatchOptions JsonMatchOptions
}

// Replay executes a request from a provider operation log against an in-memory resource provider server and asserts
// that the server's response matches the logged response.
//
// The jsonLog parameter is a verbatim JSON string such as this one:
//
//	{
//	  "method": "/pulumirpc.ResourceProvider/Create",
//	  "request": {
//	    "urn": "urn:pulumi:dev::repro-pulumi-random::random:index/randomString:RandomString::s",
//	    "properties": {
//	      "length": 1
//	    }
//	  },
//	  "response": {
//	    "id": "*",
//	    "properties": {
//	      "__meta": "{\"schema_version\":\"2\"}",
//	      "id": "*",
//	      "result": "*",
//	      "length": 1,
//	      "lower": true,
//	      "minLower": 0,
//	      "minNumeric": 0,
//	      "minSpecial": 0,
//	      "minUpper": 0,
//	      "number": true,
//	      "numeric": true,
//	      "special": true,
//	      "upper": true
//	    }
//	  }
//	}
//
// The format is the JSON encoding of the gRPC protocol used by Pulumi ResourceProvider service.
//
//	https://github.com/pulumi/pulumi/blob/master/proto/pulumi/provider.proto#L27
//
// Conveniently, the format matches what Pulumi CLI emits when invoked with PULUMI_DEBUG_GPRC:
//
//	PULUMI_DEBUG_GPRC=$PWD/log.json pulumi up
//
// This allows quickly turning fragments of the program execution trace into test cases.
//
// Instead of direct JSON equality, Replay uses AssertJSONMatchesPattern to compare the actual and expected responses.
// This allows patterns such as "*". In the above example, the random provider will generate new strings with every
// invocation and they would fail a strict equality check. Using "*" allows the test to succeed while ignoring the
// randomness.
//
// Beware possible side-effects: although Replay executes in-memory without actual gRPC sockets, replaying against an
// actual resource provider will side-effect. For example, replaying Create calls against pulumi-aws provider may try to
// create resorces in AWS. This is not an issue with side-effect-free providers such as pulumi-random, or for methods
// that do not involve cloud interaction such as Diff.
//
// Replay does not assume that the provider is a bridged provider and can be generally useful.
func Replay(t *testing.T, server pulumirpc.ResourceProviderServer, jsonLog string) {
	ReplayWithOpts(t, server, jsonLog, ReplayOptions{})
}

func ReplayWithOpts(t *testing.T, server pulumirpc.ResourceProviderServer, jsonLog string, opts ReplayOptions) {
	ctx := context.Background()
	var entry jsonLogEntry
	err := json.Unmarshal([]byte(jsonLog), &entry)
	assert.NoError(t, err)

	switch entry.Method {

	case "/pulumirpc.ResourceProvider/GetSchema":
		replay(t, entry, new(pulumirpc.GetSchemaRequest), server.GetSchema, opts)

	case "/pulumirpc.ResourceProvider/CheckConfig":
		replay(t, entry, new(pulumirpc.CheckRequest), server.CheckConfig, opts)

	case "/pulumirpc.ResourceProvider/DiffConfig":
		replay(t, entry, new(pulumirpc.DiffRequest), server.DiffConfig, opts)

	case "/pulumirpc.ResourceProvider/Configure":
		replay(t, entry, new(pulumirpc.ConfigureRequest), server.Configure, opts)

	case "/pulumirpc.ResourceProvider/Invoke":
		replay(t, entry, new(pulumirpc.InvokeRequest), server.Invoke, opts)

	// TODO StreamInvoke might need some special handling as it is a streaming RPC method.

	case "/pulumirpc.ResourceProvider/Call":
		replay(t, entry, new(pulumirpc.CallRequest), server.Call, opts)

	case "/pulumirpc.ResourceProvider/Check":
		replay(t, entry, new(pulumirpc.CheckRequest), server.Check, opts)

	case "/pulumirpc.ResourceProvider/Diff":
		replay(t, entry, new(pulumirpc.DiffRequest), server.Diff, opts)

	case "/pulumirpc.ResourceProvider/Create":
		replay(t, entry, new(pulumirpc.CreateRequest), server.Create, opts)

	case "/pulumirpc.ResourceProvider/Read":
		replay(t, entry, new(pulumirpc.ReadRequest), server.Read, opts)

	case "/pulumirpc.ResourceProvider/Update":
		replay(t, entry, new(pulumirpc.UpdateRequest), server.Update, opts)

	case "/pulumirpc.ResourceProvider/Delete":
		replay(t, entry, new(pulumirpc.DeleteRequest), server.Delete, opts)

	case "/pulumirpc.ResourceProvider/Construct":
		replay(t, entry, new(pulumirpc.ConstructRequest), server.Construct, opts)

	case "/pulumirpc.ResourceProvider/Cancel":
		_, err := server.Cancel(ctx, &emptypb.Empty{})
		assert.NoError(t, err)

	// TODO GetPluginInfo is a bit odd in that it has an Empty request, need to generealize replay() function.
	//
	// rpc GetPluginInfo(google.protobuf.Empty) returns (PluginInfo) {}

	case "/pulumirpc.ResourceProvider/Attach":
		replay(t, entry, new(pulumirpc.PluginAttach), server.Attach, opts)

	case "/pulumirpc.ResourceProvider/GetMapping":
		replay(t, entry, new(pulumirpc.GetMappingRequest), server.GetMapping, opts)

	case "/pulumirpc.ResourceProvider/GetMappings":
		replay(t, entry, new(pulumirpc.GetMappingsRequest), server.GetMappings, opts)

	default:
		t.Errorf("Unknown method: %s", entry.Method)
	}
}

// ReplaySequence is exactly like Replay, but expects jsonLog to encode a sequence of events `[e1, e2, e3]`, and will
// call Replay on each of those events in the given order.
func ReplaySequence(t *testing.T, server pulumirpc.ResourceProviderServer, jsonLog string) {
	var entries []jsonLogEntry
	err := json.Unmarshal([]byte(jsonLog), &entries)
	assert.NoError(t, err)
	for _, e := range entries {
		bytes, err := json.Marshal(e)
		assert.NoError(t, err)
		Replay(t, server, string(bytes))
	}
}

func replay[Req protoreflect.ProtoMessage, Resp protoreflect.ProtoMessage](
	t *testing.T,
	entry jsonLogEntry,
	req Req,
	serve func(context.Context, Req) (Resp, error),
	opts ReplayOptions,
) {
	ctx := context.Background()

	err := jsonpb.Unmarshal([]byte(entry.Request), req)
	assert.NoError(t, err)

	resp, err := serve(ctx, req)
	if err != nil && entry.Errors != nil {
		assert.Equal(t, *entry.Errors, err.Error())
		return
	}
	require.NoError(t, err)
	bytes, err := jsonpb.Marshal(resp)
	assert.NoError(t, err)

	var expected, actual json.RawMessage = entry.Response, bytes

	AssertJSONMatchesPatternWithOpts(t, expected, actual, opts.MatchOptions)
}

// ReplayFile executes ReplaySequence on all pulumirpc.ResourceProvider events found in the file produced with
// PULUMI_DEBUG_GPRC. For example:
//
//	PULUMI_DEBUG_GPRC=testdata/log.json pulumi up
//
// This produces the testdata/log.json file, which can then be used for Replay-style testing:
//
//	ReplayFile(t, server, "testdata/log.json")
func ReplayFile(t *testing.T, server pulumirpc.ResourceProviderServer, traceFile string) {
	bytes, err := os.ReadFile(traceFile)
	require.NoError(t, err)

	var entries []jsonLogEntry
	err = json.Unmarshal(bytes, &entries)
	require.NoError(t, err)

	count := 0
	for _, entry := range entries {
		if entry.Method == "" {
			continue
		}

		if !strings.HasPrefix(entry.Method, "/pulumirpc.ResourceProvider") {
			continue
		}
		// TODO support replaying all these method calls.
		switch entry.Method {
		case "/pulumirpc.ResourceProvider/StreamInvoke":
			continue
		case "/pulumirpc.ResourceProvider/GetPluginInfo":
			continue
		default:
			entryBytes, err := json.Marshal(entry)
			require.NoError(t, err)
			Replay(t, server, string(entryBytes))
			count++
		}
	}
	assert.Greater(t, count, 0)
}

type jsonLogEntry struct {
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Errors   *string         `json:"errors,omitempty"`
}
