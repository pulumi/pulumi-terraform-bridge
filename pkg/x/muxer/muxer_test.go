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
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	urn "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestMuxerAttach(t *testing.T) {
	t.Parallel()
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
			},
		}
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

func TestMuxerDiffConfig(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name           string
		request        *pulumirpc.DiffRequest
		response1      *pulumirpc.DiffResponse
		response2      *pulumirpc.DiffResponse
		mergedResponse *pulumirpc.DiffResponse
	}

	changeAwsRegionReq := &pulumirpc.DiffRequest{
		Urn: "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
		Olds: &structpb.Struct{Fields: map[string]*structpb.Value{
			"region":  structpb.NewStringValue("us-east-1"),
			"version": structpb.NewStringValue("6.22.0"),
		}},
		News: &structpb.Struct{Fields: map[string]*structpb.Value{
			"region":  structpb.NewStringValue("us-east-1"),
			"version": structpb.NewStringValue("6.22.0"),
		}},
		OldInputs: &structpb.Struct{Fields: map[string]*structpb.Value{
			"region":  structpb.NewStringValue("us-east-1"),
			"version": structpb.NewStringValue("6.22.0"),
		}},
	}

	changeAwsRegionResponse := &pulumirpc.DiffResponse{
		Diffs:    []string{"region"},
		Replaces: []string{"region"},
		Changes:  pulumirpc.DiffResponse_DIFF_SOME,
		DetailedDiff: map[string]*pulumirpc.PropertyDiff{
			"region": {
				InputDiff: true,
				Kind:      pulumirpc.PropertyDiff_UPDATE_REPLACE,
			},
		},
	}

	changeAwsRegionResponseCorrected := &pulumirpc.DiffResponse{
		Diffs:           []string{}, // looks like muxer normalizes this to not include replaces
		Stables:         []string{}, // looks like nils got normalized to empty list, no problem
		Replaces:        []string{"region"},
		Changes:         pulumirpc.DiffResponse_DIFF_SOME,
		HasDetailedDiff: true, // this got populated by muxer even if upstream forgets it
		DetailedDiff: map[string]*pulumirpc.PropertyDiff{
			"region": {
				InputDiff: true,
				Kind:      pulumirpc.PropertyDiff_UPDATE_REPLACE,
			},
		},
	}

	testCases := []testCase{
		{
			name:           "unimplemented server2 respects the implemented server1",
			request:        changeAwsRegionReq,
			response1:      changeAwsRegionResponse,
			mergedResponse: changeAwsRegionResponse,
		},
		{
			name:           "unimplemented server1 respects the implemented server2",
			request:        changeAwsRegionReq,
			response2:      changeAwsRegionResponse,
			mergedResponse: changeAwsRegionResponse,
		},
		{
			name:           "identical servers are treated as each of them",
			request:        changeAwsRegionReq,
			response1:      changeAwsRegionResponse,
			response2:      changeAwsRegionResponse,
			mergedResponse: changeAwsRegionResponseCorrected,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			m := &muxer{
				servers: []pulumirpc.ResourceProviderServer{
					&diffConfigServer{resp: tc.response1},
					&diffConfigServer{resp: tc.response2},
				},
			}

			actualResponse, err := m.DiffConfig(ctx, tc.request)
			require.NoError(t, err)

			require.Equal(t, tc.mergedResponse, actualResponse)
		})
	}
}

type diffConfigServer struct {
	pulumirpc.UnimplementedResourceProviderServer
	resp *pulumirpc.DiffResponse
}

func (s diffConfigServer) DiffConfig(
	ctx context.Context, req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	if s.resp != nil {
		return s.resp, nil
	}
	return s.UnimplementedResourceProviderServer.DiffConfig(ctx, req)
}

func TestConfigureInSequence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		var count atomic.Uint32
		m := &muxer{
			host: &host{},
			servers: []server{
				configure{t: t, expect: 0, counter: &count},
				configure{t: t, expect: 1, counter: &count},
				configure{t: t, expect: 2, counter: &count},
				configure{t: t, expect: 3, counter: &count},
			},
		}
		_, err := m.Configure(ctx, &pulumirpc.ConfigureRequest{})
		require.NoError(t, err)

		assert.Equal(t, uint32(4), count.Load())
	}
}

// Some providers such as pulumi-gcp will panic in PluginFramework Configure if SDKv2 Configure has
// produced errors. That is they do not expect both being called in the error case. This test checks
// that such panics are ignored and processed as expected.
func TestConfigureIgnorePanics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	m := &muxer{
		host: &host{},
		servers: []server{
			configureReturnsErrors{},
			configurePanics{},
		},
	}
	_, err := m.Configure(ctx, &pulumirpc.ConfigureRequest{})
	require.Error(t, err)
}

type configureReturnsErrors struct {
	pulumirpc.UnimplementedResourceProviderServer
}

func (x configureReturnsErrors) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return nil, fmt.Errorf("Required configuration values have not been set")
}

type configurePanics struct {
	pulumirpc.UnimplementedResourceProviderServer
}

func (x configurePanics) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	panic("Configure panics unexpectedly")
}

type configure struct {
	pulumirpc.UnimplementedResourceProviderServer
	t       *testing.T
	expect  uint32
	counter *atomic.Uint32
}

func (c configure) Configure(context.Context, *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	assert.True(c.t, c.counter.CompareAndSwap(c.expect, c.expect+1), "")
	return &pulumirpc.ConfigureResponse{}, nil
}
