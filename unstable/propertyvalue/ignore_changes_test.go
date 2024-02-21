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

package propertyvalue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestIgnoreChanges(t *testing.T) {
	old := func() resource.PropertyMap {
		return resource.PropertyMap{
			"topProp": resource.NewStringProperty("hi"),
			"listProp": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("1"),
				resource.NewStringProperty("10"),
				resource.NewStringProperty("11"),
			}),
			"mapProp": resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("0"),
				"bar": resource.NewStringProperty("1"),
			}),
			"*":   resource.NewNumberProperty(3.0),
			"old": resource.NewStringProperty("321"),
		}
	}

	new := func() resource.PropertyMap {
		return resource.PropertyMap{
			"topProp": resource.NewStringProperty("bye"),
			"listProp": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("-1"),
				resource.NewStringProperty("-10"),
				resource.NewStringProperty("-11"),
			}),
			"mapProp": resource.NewObjectProperty(resource.PropertyMap{
				"foo": resource.NewStringProperty("-0"),
				"bar": resource.NewStringProperty("-1"),
			}),
			"*":   resource.NewNumberProperty(-3.0),
			"new": resource.NewStringProperty("123"),
		}
	}

	cases := []struct {
		ignoreChanges []string
		path          string
		expectChange  bool
		notes         string
	}{
		{
			notes:         "no ignoreChanges means nothing is ignored",
			ignoreChanges: []string{},
			path:          "topProp",
			expectChange:  true,
		},
		{
			notes:         "wildcard ignores everything",
			ignoreChanges: []string{"*"},
			path:          "topProp",
		},
		{
			notes:         "ignores work even if they only match a prefix of the path",
			ignoreChanges: []string{"listProp"},
			path:          "listProp[1]",
		},
		{
			notes:         "known list element is ignored",
			ignoreChanges: []string{"listProp[1]"},
			path:          "listProp[1]",
		},
		{
			notes:         "known list element is not ignored",
			ignoreChanges: []string{"listProp[2]"},
			path:          "listProp[1]",
			expectChange:  true,
		},
		{
			notes:         "any list element is ignored",
			ignoreChanges: []string{"listProp[*]"},
			path:          "listProp[1]",
		},
		{
			notes:         "known map element is ignored",
			ignoreChanges: []string{"mapProp.foo"},
			path:          "mapProp.foo",
		},
		{
			notes:         "known map element is not ignored",
			ignoreChanges: []string{"mapProp.bar"},
			path:          "mapProp.foo",
			expectChange:  true,
		},
		{
			notes:         "any map element is ignored",
			ignoreChanges: []string{"mapProp[*]"},
			path:          "mapProp.foo",
		},
		{
			notes:         "new elements are removed if ignored",
			ignoreChanges: []string{"new"},
			path:          "new",
		},
		{
			notes:         "old elements are preserved if ignored",
			ignoreChanges: []string{"old"},
			path:          "old",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.path+":"+c.notes, func(t *testing.T) {
			old, new := old(), new()
			path, err := resource.ParsePropertyPath(c.path)
			require.NoError(t, err)

			new, err = ApplyIgnoreChanges(old, new, c.ignoreChanges)
			require.NoError(t, err)

			prev, _ := path.Get(resource.NewObjectProperty(old))
			value, _ := path.Get(resource.NewObjectProperty(new))

			if c.expectChange {
				assert.NotEqual(t, prev, value)
			} else {
				assert.Equal(t, prev, value)

			}
		})
	}
}

func TestIgnoreChangesCopiesEntries(t *testing.T) {
	olds := resource.PropertyMap{"k": resource.NewObjectProperty(resource.PropertyMap{
		"a": resource.NewStringProperty("A"),
		"b": resource.NewStringProperty("B"),
	})}
	news := resource.PropertyMap{"k": resource.NewObjectProperty(resource.PropertyMap{
		"a": resource.NewStringProperty("A"),
	})}
	news2, err := ApplyIgnoreChanges(olds, news, []string{"k[*]"})
	assert.NoError(t, err)
	assert.Equal(t, olds, news2)
}

func TestIgnoreChangesRemovesEntries(t *testing.T) {
	olds := resource.PropertyMap{"k": resource.NewObjectProperty(resource.PropertyMap{})}
	news := resource.PropertyMap{"k": resource.NewObjectProperty(resource.PropertyMap{
		"a": resource.NewStringProperty("A"),
	})}
	news2, err := ApplyIgnoreChanges(olds, news, []string{"k.a"})
	assert.NoError(t, err)
	assert.Equal(t, olds, news2)
}

func TestIgnoreChangesNestedGlob(t *testing.T) {
	olds := resource.PropertyMap{
		"k1": resource.NewObjectProperty(resource.PropertyMap{
			"*": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("v1"),
					"m2": resource.NewStringProperty("v2"),
				}),
			}),
			"not-glob": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"m3": resource.NewStringProperty("v3"),
					"m4": resource.NewStringProperty("v4"),
				}),
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("v5"),
					"m5": resource.NewStringProperty("v6"),
				}),
			}),
		}),
		"k2": resource.NewObjectProperty(resource.PropertyMap{
			"*": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("v7"),
					"m2": resource.NewStringProperty("v8"),
				}),
			}),
			"not-glob": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"m3": resource.NewStringProperty("v9"),
					"m4": resource.NewStringProperty("v10"),
				}),
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("v11"),
					"m5": resource.NewStringProperty("v12"),
				}),
			}),
		}),
	}
	news := resource.PropertyMap{
		"k1": resource.NewObjectProperty(resource.PropertyMap{
			"*": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i1"),
					"m2": resource.NewStringProperty("i2"),
				}),
			}),
			"not-glob": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"m3": resource.NewStringProperty("i3"),
					"m4": resource.NewStringProperty("i4"),
				}),
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i5"),
					"m5": resource.NewStringProperty("i6"),
				}),
			}),
		}),
		"k2": resource.NewObjectProperty(resource.PropertyMap{
			"*": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i7"),
					"m2": resource.NewStringProperty("i8"),
				}),
			}),
			"not-glob": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"m3": resource.NewStringProperty("i9"),
					"m4": resource.NewStringProperty("i10"),
				}),
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i11"),
					"m5": resource.NewStringProperty("i12"),
				}),
			}),
		}),
	}

	path := `["*"]["not-glob"][1].m5`

	news, err := ApplyIgnoreChanges(olds, news, []string{path})
	assert.NoError(t, err)
	assert.Equal(t, resource.PropertyMap{
		"k1": resource.NewObjectProperty(resource.PropertyMap{
			"*": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i1"),
					"m2": resource.NewStringProperty("i2"),
				}),
			}),
			"not-glob": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"m3": resource.NewStringProperty("i3"),
					"m4": resource.NewStringProperty("i4"),
				}),
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i5"),
					"m5": resource.NewStringProperty("v6"),
				}),
			}),
		}),
		"k2": resource.NewObjectProperty(resource.PropertyMap{
			"*": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i7"),
					"m2": resource.NewStringProperty("i8"),
				}),
			}),
			"not-glob": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewObjectProperty(resource.PropertyMap{
					"m3": resource.NewStringProperty("i9"),
					"m4": resource.NewStringProperty("i10"),
				}),
				resource.NewObjectProperty(resource.PropertyMap{
					"*":  resource.NewStringProperty("i11"),
					"m5": resource.NewStringProperty("v12"),
				}),
			}),
		}),
	}, news)
}
