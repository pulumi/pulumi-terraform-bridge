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
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	urn "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func TestAttach(t *testing.T) {
	req := &pulumirpc.PluginAttach{Address: "test"}
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		h := &host{}
		m := &muxer{host: h}
		_, err := m.Attach(ctx, req)
		assert.NoError(t, err)
		assert.NotZero(t, m.host)
		assert.True(t, h.closed)
	})

	t.Run("dispatch", func(t *testing.T) {
		h := &host{}
		m := &muxer{
			host: h,
			servers: []server{
				&attach{t: t, expected: "test"},
				&attach{t: t, expected: "test"},
			}}
		_, err := m.Attach(ctx, req)
		assert.NoError(t, err)
		for i, s := range m.servers {
			assert.Equalf(t, 1, s.(*attach).called, "i = %d", i)
		}
		assert.NotZero(t, m.host)
		assert.True(t, h.closed)
	})
}

type attach struct {
	pulumirpc.UnimplementedResourceProviderServer

	t        *testing.T
	expected string
	called   int
}

func (s *attach) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	assert.Equal(s.t, s.expected, req.Address)
	s.called++
	return &emptypb.Empty{}, nil
}

type host struct{ closed bool }

func (h *host) Close() error {
	if h.closed {
		return fmt.Errorf("host already closed")
	}
	h.closed = true
	return nil
}

func (h *host) Log(context.Context, diag.Severity, urn.URN, string) error {
	if h.closed {
		return fmt.Errorf("cannot log against a closed host")
	}
	return nil
}
