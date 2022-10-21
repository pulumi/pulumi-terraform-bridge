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
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/stretchr/testify/assert"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Replays a provider operation log captured by PULUMI_DEBUG_GPRC=log.json against a server and asserts that the
// response matches the one from the log.
func Replay(t *testing.T, server pulumirpc.ResourceProviderServer, jsonLog string) {
	ctx := context.Background()

	var entry jsonLogEntry
	err := json.Unmarshal([]byte(jsonLog), &entry)
	assert.NoError(t, err)

	switch entry.Method {
	case "/pulumirpc.ResourceProvider/Check":
		var req pulumirpc.CheckRequest

		err := jsonpb.Unmarshal(bytes.NewBuffer([]byte(entry.Request)), &req)
		assert.NoError(t, err)

		resp, err := server.Check(ctx, &req)
		assert.NoError(t, err)

		m := jsonpb.Marshaler{}
		buf := bytes.Buffer{}
		err = m.Marshal(&buf, resp)
		assert.NoError(t, err)

		var expected, actual json.RawMessage = entry.Response, buf.Bytes()
		assert.Equal(t, pretty(t, expected), pretty(t, actual))

	case "/pulumirpc.ResourceProvider/Diff":
		var req pulumirpc.DiffRequest

		err := jsonpb.Unmarshal(bytes.NewBuffer([]byte(entry.Request)), &req)
		assert.NoError(t, err)

		resp, err := server.Diff(ctx, &req)
		assert.NoError(t, err)

		m := jsonpb.Marshaler{}
		buf := bytes.Buffer{}
		err = m.Marshal(&buf, resp)
		assert.NoError(t, err)

		var expected, actual json.RawMessage = entry.Response, buf.Bytes()
		assert.Equal(t, pretty(t, expected), pretty(t, actual))

	default:
		t.Errorf("Unknown method: %s", entry.Method)
	}
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
