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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Replays a provider operation log captured by PULUMI_DEBUG_GPRC=log.json against a server and asserts that the
// response matches the one from the log.
func Replay(t *testing.T, server pulumirpc.ResourceProviderServer, jsonLog string) {
	var entry jsonLogEntry
	err := json.Unmarshal([]byte(jsonLog), &entry)
	assert.NoError(t, err)

	t.Logf(entry.Method)

	switch entry.Method {
	case "/pulumirpc.ResourceProvider/Check":
		replay(t, entry, new(pulumirpc.CheckRequest), server.Check)

	case "/pulumirpc.ResourceProvider/Configure":
		replay(t, entry, new(pulumirpc.ConfigureRequest), server.Configure)

	case "/pulumirpc.ResourceProvider/Create":
		replay(t, entry, new(pulumirpc.CreateRequest), server.Create)

	case "/pulumirpc.ResourceProvider/Delete":
		replay(t, entry, new(pulumirpc.DeleteRequest), server.Delete)

	case "/pulumirpc.ResourceProvider/Diff":
		replay(t, entry, new(pulumirpc.DiffRequest), server.Diff)

	case "/pulumirpc.ResourceProvider/Invoke":
		replay(t, entry, new(pulumirpc.InvokeRequest), server.Invoke)

	case "/pulumirpc.ResourceProvider/Read":
		replay(t, entry, new(pulumirpc.ReadRequest), server.Read)

	case "/pulumirpc.ResourceProvider/Update":
		replay(t, entry, new(pulumirpc.UpdateRequest), server.Update)

	default:
		t.Errorf("Unknown method: %s", entry.Method)
	}
}

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

func NewCreateRequest(t *testing.T, encoded string) *pulumirpc.CreateRequest {
	return newRequest(t, new(pulumirpc.CreateRequest), encoded)
}

func newRequest[Req proto.Message](t *testing.T, req Req, jsonRequest string) Req {
	err := jsonpb.Unmarshal(bytes.NewBuffer([]byte(jsonRequest)), req)
	require.NoError(t, err)
	return req
}

func ParseResponse[Resp proto.Message, Parsed any](t *testing.T, resp Resp, parsed Parsed) Parsed {
	m := jsonpb.Marshaler{}
	buf := bytes.Buffer{}
	err := m.Marshal(&buf, resp)
	require.NoError(t, err)
	err = json.Unmarshal(buf.Bytes(), parsed)
	require.NoError(t, err)
	return parsed
}

func replay[Req proto.Message, Resp proto.Message](
	t *testing.T,
	entry jsonLogEntry,
	req Req,
	serve func(context.Context, Req) (Resp, error),
) {
	ctx := context.Background()

	err := jsonpb.Unmarshal(bytes.NewBuffer([]byte(entry.Request)), req)
	assert.NoError(t, err)

	resp, err := serve(ctx, req)
	require.NoError(t, err)

	m := jsonpb.Marshaler{}
	buf := bytes.Buffer{}
	err = m.Marshal(&buf, resp)
	assert.NoError(t, err)

	var expected, actual json.RawMessage = entry.Response, buf.Bytes()
	assert.Equal(t, pretty(t, expected), pretty(t, actual))
}

// Replays all the events from traceFile=log.json captured by PULUMI_DEBUG_GPRC=log.json against a given server.
func ReplayTraceFile(t *testing.T, server pulumirpc.ResourceProviderServer, traceFile string) {
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
		case "/pulumirpc.ResourceProvider/Configure":
			continue
		case "/pulumirpc.ResourceProvider/CheckConfigure":
			continue
		case "/pulumirpc.ResourceProvider/GetPluginInfo":
			continue
		case "/pulumirpc.ResourceProvider/DiffConfig":
			continue
		case "/pulumirpc.ResourceProvider/CheckConfig":
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
}

func pretty(t *testing.T, raw json.RawMessage) string {
	buf := bytes.Buffer{}
	err := json.Indent(&buf, []byte(raw), "", "  ")
	assert.NoError(t, err)
	return buf.String()
}
