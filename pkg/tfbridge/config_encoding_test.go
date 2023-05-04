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

package tfbridge

import (
	"fmt"
	"testing"

	structpb "google.golang.org/protobuf/types/known/structpb"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	sch "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

func TestConfigEncoding(t *testing.T) {
	type testCase struct {
		ty shim.ValueType
		v  *structpb.Value
		pv resource.PropertyValue
	}

	knownKey := "mykey"

	makeEnc := func(ty shim.ValueType) *ConfigEncoding {
		return NewConfigEncoding(
			&sch.SchemaMap{knownKey: (&sch.Schema{Type: ty}).Shim()},
			map[string]*SchemaInfo{
				knownKey: {
					// Avoid name mangling for this test, make sure TF and Pulumi name of the known
					// property are the same.
					Name: knownKey,
				},
			},
		)
	}

	makeValue := func(x any) *structpb.Value {
		vv, err := structpb.NewValue(x)
		assert.NoErrorf(t, err, "structpb.NewValue failed")
		return vv
	}

	checkMarshal := func(t *testing.T, tc testCase) {
		enc := makeEnc(tc.ty)
		s, err := enc.MarshalProperties(resource.PropertyMap{
			resource.PropertyKey(knownKey): tc.pv,
		})
		assert.NoError(t, err)
		assert.NotNil(t, s)
		x, ok := s.Fields[knownKey]
		assert.True(t, ok)
		expectedJSON, err := tc.v.MarshalJSON()
		assert.NoError(t, err)
		actualJSON, err := x.MarshalJSON()
		assert.NoError(t, err)
		t.Logf("%s", actualJSON)
		assert.Equal(t, string(expectedJSON), string(actualJSON))
	}

	checkUnmarshal := func(t *testing.T, tc testCase) {
		enc := makeEnc(tc.ty)
		pv, err := enc.unmarshalPropertyValue(resource.PropertyKey(knownKey), tc.v)
		assert.NoError(t, err)
		assert.NotNil(t, pv)
		assert.Equal(t, tc.pv, *pv)
	}

	turnaroundTestCases := []testCase{
		{
			shim.TypeBool,
			makeValue(`true`),
			resource.NewBoolProperty(true),
		},
		{
			shim.TypeBool,
			makeValue(`false`),
			resource.NewBoolProperty(false),
		},
		{
			shim.TypeInt,
			makeValue(`0`),
			resource.NewNumberProperty(0),
		},
		{
			shim.TypeInt,
			makeValue(`42`),
			resource.NewNumberProperty(42),
		},
		{
			shim.TypeFloat,
			makeValue(`0`),
			resource.NewNumberProperty(0.0),
		},
		{
			shim.TypeFloat,
			makeValue(`42.5`),
			resource.NewNumberProperty(42.5),
		},
		{
			shim.TypeString,
			structpb.NewStringValue(""),
			resource.NewStringProperty(""),
		},
		{
			shim.TypeString,
			structpb.NewStringValue("hello"),
			resource.NewStringProperty("hello"),
		},
		{
			shim.TypeList,
			makeValue(`[]`),
			resource.NewArrayProperty([]resource.PropertyValue{}),
		},
		{
			shim.TypeList,
			makeValue(`["hello","there"]`),
			resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
				resource.NewStringProperty("there"),
			}),
		},
		{
			shim.TypeSet,
			makeValue(`[]`),
			resource.NewArrayProperty([]resource.PropertyValue{}),
		},
		{
			shim.TypeSet,
			makeValue(`["hello","there"]`),
			resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("hello"),
				resource.NewStringProperty("there"),
			}),
		},
		{
			shim.TypeMap,
			makeValue(`{}`),
			resource.NewObjectProperty(resource.PropertyMap{}),
		},
		{
			shim.TypeMap,
			makeValue(`{"key":"value"}`),
			resource.NewObjectProperty(resource.PropertyMap{
				"key": resource.NewStringProperty("value"),
			}),
		},
	}

	t.Run("turnaround", func(t *testing.T) {
		for i, tc := range turnaroundTestCases {
			tc := tc

			t.Run(fmt.Sprintf("UnmarshalPropertyValue/%d", i), func(t *testing.T) {
				checkUnmarshal(t, tc)
			})

			t.Run(fmt.Sprintf("MarshalPropertyValue/%d", i), func(t *testing.T) {
				checkMarshal(t, tc)
			})
		}
	})

	t.Run("zero_values", func(t *testing.T) {
		// Historically the encoding was able to convert empty strings into type-appropriate zero values.
		cases := []testCase{
			{
				shim.TypeBool,
				makeValue(""),
				resource.NewBoolProperty(false),
			},
			{
				shim.TypeFloat,
				makeValue(""),
				resource.NewNumberProperty(0.),
			},
			{
				shim.TypeInt,
				makeValue(""),
				resource.NewNumberProperty(0),
			},
			{
				shim.TypeString,
				makeValue(""),
				resource.NewStringProperty(""),
			},
			{
				shim.TypeMap,
				makeValue(""),
				resource.NewObjectProperty(make(resource.PropertyMap)),
			},
			{
				shim.TypeList,
				makeValue(""),
				resource.NewArrayProperty([]resource.PropertyValue{}),
			},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(fmt.Sprintf("%v", tc.ty), func(t *testing.T) {
				checkUnmarshal(t, tc)
			})
		}
	})

	t.Run("computed", func(t *testing.T) {
		unk := makeValue(plugin.UnknownStringValue)

		for i, tc := range turnaroundTestCases {
			tc := tc

			t.Run(fmt.Sprintf("UnmarshalPropertyValue/%d", i), func(t *testing.T) {
				// Unknown sentinel unmarshals to a Computed with a type-appropriate zero value.
				checkUnmarshal(t, testCase{
					tc.ty,
					unk,
					resource.MakeComputed(makeEnc(tc.ty).zeroValue(tc.ty)),
				})
			})

			t.Run(fmt.Sprintf("MarshalPropertyValue/%d", i), func(t *testing.T) {
				// When there any unknowns at all in the config, unk sentinel is sent by the engine.
				checkMarshal(t, testCase{
					tc.ty,
					unk,
					resource.MakeComputed(tc.pv),
				})
			})
		}

		// Checking a few extra cases for nested secrets.
		nestedUnkownTestCases := []testCase{
			{
				shim.TypeList,
				unk,
				resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("hello"),
					resource.MakeComputed(resource.NewNumberProperty(0)),
				}),
			},
			{
				shim.TypeSet,
				unk,
				resource.NewArrayProperty([]resource.PropertyValue{
					resource.MakeComputed(resource.NewBoolProperty(false)),
					resource.NewStringProperty("there"),
				}),
			},
			{
				shim.TypeMap,
				unk,
				resource.NewObjectProperty(resource.PropertyMap{
					"key": resource.MakeComputed(resource.NewStringProperty("")),
				}),
			},
		}
		for i, tc := range nestedUnkownTestCases {
			tc := tc
			t.Run(fmt.Sprintf("nested/MarshalPropertyValue/%d", i), func(t *testing.T) {
				checkMarshal(t, tc)
			})
		}
	})

	t.Run("secret", func(t *testing.T) {
		// Unmarshalling happens with KeepSecrets=false, replacing them with the underlying values. This case
		// does not need to be tested.
		//
		// Marhalling however supports sending secrets back to the engine, intending to mark values as secret
		// that happen on paths that are declared as secret in the schema. Due to the limitation of the
		// JSON-in-proto-encoding, secrets are communicated imprecisely as an approximation: if any nested
		// element of a property is secret, the entire property is marshalled as secret.

		var secretCases []testCase

		pbSecret := func(v *structpb.Value) *structpb.Value {
			return structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
				"4dabf18193072939515e22adb298388d": makeValue("1b47061264138c4ac30d75fd1eb44270"),
				"value":                            v,
			}})
		}

		for _, tc := range turnaroundTestCases {
			secretCases = append(secretCases, testCase{
				tc.ty,
				pbSecret(tc.v),
				resource.MakeSecret(tc.pv),
			})
		}

		for i, tc := range secretCases {
			tc := tc

			t.Run(fmt.Sprintf("secret/MarshalPropertyValue/%d", i), func(t *testing.T) {
				checkMarshal(t, tc)
			})

			t.Run(fmt.Sprintf("secret/UnmarshalPropertyValue/%d", i), func(t *testing.T) {
				// Unmarshallin will remove secrts, so the expected value needs to be modified.
				tc.pv = tc.pv.SecretValue().Element
				checkUnmarshal(t, tc)
			})
		}

		// Checking a few extra cases for nested secrets.
		nestedSecretCases := []testCase{
			{
				shim.TypeList,
				pbSecret(makeValue(`["hello",1]`)),
				resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("hello"),
					resource.MakeSecret(resource.NewNumberProperty(1)),
				}),
			},
			{
				shim.TypeSet,
				pbSecret(makeValue(`[false,"there"]`)),
				resource.NewArrayProperty([]resource.PropertyValue{
					resource.MakeSecret(resource.NewBoolProperty(false)),
					resource.NewStringProperty("there"),
				}),
			},
			{
				shim.TypeMap,
				pbSecret(makeValue(`{"key":""}`)),
				resource.NewObjectProperty(resource.PropertyMap{
					"key": resource.MakeSecret(resource.NewStringProperty("")),
				}),
			},
		}
		for i, tc := range nestedSecretCases {
			tc := tc
			t.Run(fmt.Sprintf("nested/MarshalPropertyValue/%d", i), func(t *testing.T) {
				checkMarshal(t, tc)
			})
		}

		t.Run("tolerate secrets in Configure", func(t *testing.T) {
			// This is a bit of a histirocal quirk: the engine may send secrets to Configure before
			// receiving the response from Configure indicating that the provider does not want to receive
			// secrets. These are simply ignored. The engine does not currently send secrets to CheckConfig.
			// The engine does take care of making sure the secrets are stored as such in the statefile.
			//
			// Check here that unmarshalilng such values removes the secrets.
			checkUnmarshal(t, testCase{
				shim.TypeMap,
				pbSecret(makeValue(`{"key":"val"}`)),
				resource.NewObjectProperty(resource.PropertyMap{
					"key": resource.NewStringProperty("val"),
				}),
			})
		})
	})

	// NOTE about the PropertyValue cases not tested here.
	//
	// NewAssetProperty, NewArchiveProperty are skipped because MarshalOptions sets RejectAssets: true, which will
	// cause the provider to reject such values with an error. They are not currently supported.
	//
	// NewNullProperty case is skipped because of SkipNulls: true in MarshalOptions removes them.
	//
	// NewOutputProperty case is skipped because bridged providers respond to Configure with AcceptOutputs: false,
	// and the engine never sends first-class outputs to bridged providers.
	//
	// NewResourceReferenceProperty is skipped because Configure responds with AcceptResources: false and the engine
	// never sends these either.
}
