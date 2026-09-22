// Copyright 2016-2024, Pulumi Corporation.
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

package sdkv2

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	rapidgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/rapid"
)

// TestPlanResourceChangeMatchesUpstream is a differential check of the bridge's
// planResourceChange against the upstream GRPCProviderServer.PlanResourceChange
// it reproduces. Both are run on the same generated resource schema and prior
// and proposed states, and must agree on the planned state, planned private
// state, and whether an error is returned.
//
// The ignoreChanges path is not checked: it has no upstream counterpart.
func TestPlanResourceChangeMatchesUpstream(t *testing.T) {
	t.Parallel()
	const typeName = "test_resource"

	rapid.Check(t, func(t *rapid.T) {
		res := rapidgen.ResourceProperGen(3).Draw(t, "resource")
		if err := res.InternalValidate(nil, false); err != nil {
			t.Skipf("invalid schema: %v", err)
		}
		ty := res.CoreConfigSchema().ImpliedType()

		prior := stateGen(ty, false).Draw(t, "prior")
		proposed := proposedGen(ty, prior).Draw(t, "proposed")

		tf := &schema.Provider{ResourcesMap: map[string]*schema.Resource{typeName: res}}
		s := &grpcServer{gserver: schema.NewGRPCProviderServer(tf), provider: tf}
		ctx := context.Background()

		// The SDK's planning of sets of nested blocks is not fully deterministic:
		// the same inputs occasionally plan differently. A mismatch is only
		// reported when it reproduces.
		var got, want planOutcome
		for attempt := 0; attempt < 3; attempt++ {
			gotState, gotPrivate, _, gotErr := s.planResourceChange(
				ctx, res, proposed, prior, proposed, nil, nil, nil)
			got = planOutcome{gotState, privateRoundTrip(t, gotPrivate), gotErr}

			resp, err := s.gserver.PlanResourceChange(ctx, &tfprotov5.PlanResourceChangeRequest{
				TypeName:         typeName,
				PriorState:       dynamicValue(t, prior, ty),
				ProposedNewState: dynamicValue(t, proposed, ty),
				Config:           dynamicValue(t, proposed, ty),
			})
			require.NoError(t, err)
			want = planOutcome{err: handleDiagnostics(ctx, resp.Diagnostics, nil)}
			if want.err == nil {
				want.state, err = msgpack.Unmarshal(resp.PlannedState.MsgPack, ty)
				require.NoError(t, err)
				want.private = privateFromJSON(t, resp.PlannedPrivate)
			}

			if got.equal(want) {
				return
			}
		}
		t.Fatalf("bridge and upstream plans differ\nupstream: %s\nbridge:   %s", want, got)
	})
}

type planOutcome struct {
	state   cty.Value
	private map[string]interface{}
	err     error
}

func (o planOutcome) equal(other planOutcome) bool {
	if o.err != nil || other.err != nil {
		return (o.err != nil) == (other.err != nil)
	}
	return normalizeCtyValue(o.state).RawEquals(normalizeCtyValue(other.state)) &&
		reflect.DeepEqual(o.private, other.private)
}

func (o planOutcome) String() string {
	if o.err != nil {
		return fmt.Sprintf("error: %v", o.err)
	}
	return fmt.Sprintf("state %s private %v", o.state.GoString(), o.private)
}

func dynamicValue(t require.TestingT, v cty.Value, ty cty.Type) *tfprotov5.DynamicValue {
	b, err := msgpack.Marshal(v, ty)
	require.NoError(t, err)
	return &tfprotov5.DynamicValue{MsgPack: b}
}

func privateFromJSON(t require.TestingT, b []byte) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

// privateRoundTrip passes the bridge's planned private state through JSON so
// that it compares equal to upstream's JSON-decoded form.
func privateRoundTrip(t require.TestingT, m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return privateFromJSON(t, b)
}

// stateGen generates a resource state of object type ty: either null or a
// known object whose attributes are drawn by ctyValueGen.
func stateGen(ty cty.Type, allowUnknown bool) *rapid.Generator[cty.Value] {
	return rapid.OneOf(rapid.Just(cty.NullVal(ty)), objectGen(ty, allowUnknown))
}

// proposedGen generates a proposed state: null, a fresh object, or a copy of
// the prior state with one attribute redrawn. The last case makes no-op and
// single-attribute plans common enough to exercise the SDK's equivalence
// short-cuts.
func proposedGen(ty cty.Type, prior cty.Value) *rapid.Generator[cty.Value] {
	gens := []*rapid.Generator[cty.Value]{rapid.Just(cty.NullVal(ty)), objectGen(ty, true)}
	if !prior.IsNull() {
		gens = append(gens, rapid.Custom[cty.Value](func(t *rapid.T) cty.Value {
			attrs := prior.AsValueMap()
			name := rapid.SampledFrom(sortedKeys(attrs)).Draw(t, "attr")
			attrs[name] = attrValueGen(name, ty.AttributeType(name), true).Draw(t, name)
			return cty.ObjectVal(attrs)
		}))
	}
	return rapid.OneOf(gens...)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func objectGen(ty cty.Type, allowUnknown bool) *rapid.Generator[cty.Value] {
	attrTypes := ty.AttributeTypes()
	if len(attrTypes) == 0 {
		return rapid.Just(cty.EmptyObjectVal)
	}
	return rapid.Custom[cty.Value](func(t *rapid.T) cty.Value {
		attrs := make(map[string]cty.Value, len(attrTypes))
		for _, name := range sortedKeys(attrTypes) {
			attrs[name] = attrValueGen(name, attrTypes[name], allowUnknown).Draw(t, name)
		}
		return cty.ObjectVal(attrs)
	})
}

// attrValueGen generates the value of one object attribute. The generated
// schemas only name attributes "f1" and "f2", so an attribute named
// "timeouts" is always the SDK's timeouts block.
func attrValueGen(name string, ty cty.Type, allowUnknown bool) *rapid.Generator[cty.Value] {
	if name == schema.TimeoutsConfigKey {
		return timeoutsGen(ty)
	}
	return ctyValueGen(ty, allowUnknown)
}

// timeoutsGen generates the timeouts block: null, or each timeout either null
// or a valid duration.
func timeoutsGen(ty cty.Type) *rapid.Generator[cty.Value] {
	return rapid.Custom[cty.Value](func(t *rapid.T) cty.Value {
		if rapid.Bool().Draw(t, "nullTimeouts") {
			return cty.NullVal(ty)
		}
		attrTypes := ty.AttributeTypes()
		attrs := make(map[string]cty.Value, len(attrTypes))
		for _, name := range sortedKeys(attrTypes) {
			attrs[name] = rapid.SampledFrom([]cty.Value{
				cty.NullVal(cty.String), cty.StringVal("30s"), cty.StringVal("1m"),
			}).Draw(t, name)
		}
		return cty.ObjectVal(attrs)
	})
}

// ctyValueGen generates values that conform to ty. Nulls are always possible;
// unknowns only when allowUnknown is set. Scalars draw from a small alphabet so
// that prior and proposed values often coincide and exercise no-op planning.
func ctyValueGen(ty cty.Type, allowUnknown bool) *rapid.Generator[cty.Value] {
	return rapid.Custom[cty.Value](func(t *rapid.T) cty.Value {
		switch rapid.IntRange(0, 5).Draw(t, "kind") {
		case 0:
			return cty.NullVal(ty)
		case 1:
			if allowUnknown {
				return cty.UnknownVal(ty)
			}
		}
		elemGen := func() *rapid.Generator[cty.Value] {
			return ctyValueGen(ty.ElementType(), allowUnknown)
		}
		switch {
		case ty == cty.String:
			return cty.StringVal(rapid.SampledFrom([]string{"", "a", "b"}).Draw(t, "string"))
		case ty == cty.Number:
			return cty.NumberIntVal(int64(rapid.IntRange(-2, 2).Draw(t, "number")))
		case ty == cty.Bool:
			return cty.BoolVal(rapid.Bool().Draw(t, "bool"))
		case ty.IsListType():
			elems := rapid.SliceOfN(elemGen(), 0, 2).Draw(t, "list")
			if len(elems) == 0 {
				return cty.ListValEmpty(ty.ElementType())
			}
			return cty.ListVal(elems)
		case ty.IsSetType():
			elems := rapid.SliceOfN(elemGen(), 0, 2).Draw(t, "set")
			if len(elems) == 0 {
				return cty.SetValEmpty(ty.ElementType())
			}
			return cty.SetVal(elems)
		case ty.IsMapType():
			keyGen := rapid.SampledFrom([]string{"k1", "k2"})
			elems := rapid.MapOfN(keyGen, elemGen(), 0, 2).Draw(t, "map")
			if len(elems) == 0 {
				return cty.MapValEmpty(ty.ElementType())
			}
			return cty.MapVal(elems)
		case ty.IsObjectType():
			return objectGen(ty, allowUnknown).Draw(t, "object")
		}
		contract.Failf("unsupported cty type: %s", ty.GoString())
		return cty.NilVal
	})
}
