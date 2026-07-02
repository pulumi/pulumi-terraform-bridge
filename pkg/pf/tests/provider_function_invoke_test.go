// Copyright 2016-2026, Pulumi Corporation.
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

package tfbridgetests

import (
	"testing"

	testutils "github.com/pulumi/providertest/replay"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

// Provider-defined functions are invoked like data sources but route through the
// Terraform CallFunction RPC. Functions are pure: none of these calls configure the
// provider first.
func TestProviderFunctionInvoke(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)

	t.Run("variadic string result", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/concat:concat",
		    "args": {
		      "separator": "-",
		      "parts": ["a", "b", "c"]
		    }
		  },
		  "response": {
		    "return": {
		      "result": "a-b-c"
		    }
		  }
		}`)
	})

	t.Run("variadic with no trailing arguments", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/concat:concat",
		    "args": {
		      "separator": "-"
		    }
		  },
		  "response": {
		    "return": {
		      "result": ""
		    }
		  }
		}`)
	})

	t.Run("object result", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/parseId:parseId",
		    "args": {
		      "id": "foo/bar"
		    }
		  },
		  "response": {
		    "return": {
		      "prefix": "foo",
		      "suffix": "bar"
		    }
		  }
		}`)
	})

	t.Run("argument-scoped function error", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/parseId:parseId",
		    "args": {
		      "id": "malformed"
		    }
		  },
		  "response": {
		    "return": {},
		    "failures": [
		      {
		        "property": "id",
		        "reason": "function \"parse_id\": expected an id of the form \"prefix/suffix\""
		      }
		    ]
		  }
		}`)
	})

	t.Run("null allowed and omitted", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/nullableDefault:nullableDefault",
		    "args": {}
		  },
		  "response": {
		    "return": {
		      "result": "default"
		    }
		  }
		}`)
	})

	t.Run("null allowed and provided", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/nullableDefault:nullableDefault",
		    "args": {
		      "value": "explicit"
		    }
		  },
		  "response": {
		    "return": {
		      "result": "explicit"
		    }
		  }
		}`)
	})

	t.Run("missing required argument", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/concat:concat",
		    "args": {
		      "parts": ["a"]
		    }
		  },
		  "response": {
		    "return": {},
		    "failures": [
		      {
		        "property": "separator",
		        "reason": "function \"concat\" requires a non-null value for argument \"separator\""
		      }
		    ]
		  }
		}`)
	})

	t.Run("unexpected argument", func(t *testing.T) {
		testutils.Replay(t, server, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "testbridge:index/concat:concat",
		    "args": {
		      "separator": "-",
		      "bogus": "x"
		    }
		  },
		  "response": {
		    "return": {},
		    "failures": [
		      {
		        "property": "bogus",
		        "reason": "unexpected argument for function \"concat\""
		      }
		    ]
		  }
		}`)
	})
}
