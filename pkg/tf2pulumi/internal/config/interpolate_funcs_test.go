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

package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"path/filepath"

	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/bcrypt"
)

func TestInterpolateFuncZipMap(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${zipmap(var.list, var.list2)}`,
				map[string]interface{}{
					"Hello": "bar",
					"World": "baz",
				},
				false,
			},
			{
				`${zipmap(var.list, var.nonstrings)}`,
				map[string]interface{}{
					"Hello": []interface{}{"bar", "baz"},
					"World": []interface{}{"boo", "foo"},
				},
				false,
			},
			{
				`${zipmap(var.nonstrings, var.list2)}`,
				nil,
				true,
			},
			{
				`${zipmap(var.list, var.differentlengthlist)}`,
				nil,
				true,
			},
		},
		Vars: map[string]ast.Variable{
			"var.list": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type:  ast.TypeString,
						Value: "Hello",
					},
					{
						Type:  ast.TypeString,
						Value: "World",
					},
				},
			},
			"var.list2": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type:  ast.TypeString,
						Value: "bar",
					},
					{
						Type:  ast.TypeString,
						Value: "baz",
					},
				},
			},
			"var.differentlengthlist": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type:  ast.TypeString,
						Value: "bar",
					},
					{
						Type:  ast.TypeString,
						Value: "baz",
					},
					{
						Type:  ast.TypeString,
						Value: "boo",
					},
				},
			},
			"var.nonstrings": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type: ast.TypeList,
						Value: []ast.Variable{
							{
								Type:  ast.TypeString,
								Value: "bar",
							},
							{
								Type:  ast.TypeString,
								Value: "baz",
							},
						},
					},
					{
						Type: ast.TypeList,
						Value: []ast.Variable{
							{
								Type:  ast.TypeString,
								Value: "boo",
							},
							{
								Type:  ast.TypeString,
								Value: "foo",
							},
						},
					},
				},
			},
		},
	})
}

func TestInterpolateFuncList(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// empty input returns empty list
			{
				`${list()}`,
				[]interface{}{},
				false,
			},

			// single input returns list of length 1
			{
				`${list("hello")}`,
				[]interface{}{"hello"},
				false,
			},

			// two inputs returns list of length 2
			{
				`${list("hello", "world")}`,
				[]interface{}{"hello", "world"},
				false,
			},

			// not a string input gives error
			{
				`${list("hello", 42)}`,
				nil,
				true,
			},

			// list of lists
			{
				`${list("${var.list}", "${var.list2}")}`,
				[]interface{}{[]interface{}{"Hello", "World"}, []interface{}{"bar", "baz"}},
				false,
			},

			// list of maps
			{
				`${list("${var.map}", "${var.map2}")}`,
				[]interface{}{map[string]interface{}{"key": "bar"}, map[string]interface{}{"key2": "baz"}},
				false,
			},

			// error on a heterogeneous list
			{
				`${list("first", "${var.list}")}`,
				nil,
				true,
			},
		},
		Vars: map[string]ast.Variable{
			"var.list": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type:  ast.TypeString,
						Value: "Hello",
					},
					{
						Type:  ast.TypeString,
						Value: "World",
					},
				},
			},
			"var.list2": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type:  ast.TypeString,
						Value: "bar",
					},
					{
						Type:  ast.TypeString,
						Value: "baz",
					},
				},
			},

			"var.map": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"key": {
						Type:  ast.TypeString,
						Value: "bar",
					},
				},
			},
			"var.map2": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"key2": {
						Type:  ast.TypeString,
						Value: "baz",
					},
				},
			},
		},
	})
}

func TestInterpolateFuncMax(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${max()}`,
				nil,
				true,
			},

			{
				`${max("")}`,
				nil,
				true,
			},

			{
				`${max(-1, 0, 1)}`,
				"1",
				false,
			},

			{
				`${max(1, 0, -1)}`,
				"1",
				false,
			},

			{
				`${max(-1, -2)}`,
				"-1",
				false,
			},

			{
				`${max(-1)}`,
				"-1",
				false,
			},
		},
	})
}

func TestInterpolateFuncMin(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${min()}`,
				nil,
				true,
			},

			{
				`${min("")}`,
				nil,
				true,
			},

			{
				`${min(-1, 0, 1)}`,
				"-1",
				false,
			},

			{
				`${min(1, 0, -1)}`,
				"-1",
				false,
			},

			{
				`${min(-1, -2)}`,
				"-2",
				false,
			},

			{
				`${min(-1)}`,
				"-1",
				false,
			},
		},
	})
}

func TestInterpolateFuncPow(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${pow(1, 0)}`,
				"1",
				false,
			},
			{
				`${pow(1, 1)}`,
				"1",
				false,
			},

			{
				`${pow(2, 0)}`,
				"1",
				false,
			},
			{
				`${pow(2, 1)}`,
				"2",
				false,
			},
			{
				`${pow(3, 2)}`,
				"9",
				false,
			},
			{
				`${pow(-3, 2)}`,
				"9",
				false,
			},
			{
				`${pow(2, -2)}`,
				"0.25",
				false,
			},
			{
				`${pow(0, 2)}`,
				"0",
				false,
			},
			{
				`${pow("invalid-input", 2)}`,
				nil,
				true,
			},
			{
				`${pow(2, "invalid-input")}`,
				nil,
				true,
			},
			{
				`${pow(2)}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncFloor(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${floor()}`,
				nil,
				true,
			},

			{
				`${floor("")}`,
				nil,
				true,
			},

			{
				`${floor("-1.3")}`, // there appears to be a AST bug where the parsed argument ends up being -1 without the "s
				"-2",
				false,
			},

			{
				`${floor(1.7)}`,
				"1",
				false,
			},
		},
	})
}

func TestInterpolateFuncCeil(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${ceil()}`,
				nil,
				true,
			},

			{
				`${ceil("")}`,
				nil,
				true,
			},

			{
				`${ceil(-1.8)}`,
				"-1",
				false,
			},

			{
				`${ceil(1.2)}`,
				"2",
				false,
			},
		},
	})
}

func TestInterpolateFuncLog(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${log(1, 10)}`,
				"0",
				false,
			},
			{
				`${log(10, 10)}`,
				"1",
				false,
			},

			{
				`${log(0, 10)}`,
				"-Inf",
				false,
			},
			{
				`${log(10, 0)}`,
				"-0",
				false,
			},
		},
	})
}

func TestInterpolateFuncChomp(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${chomp()}`,
				nil,
				true,
			},

			{
				`${chomp("hello world")}`,
				"hello world",
				false,
			},

			{
				fmt.Sprintf(`${chomp("%s")}`, "goodbye\ncruel\nworld"),
				"goodbye\ncruel\nworld",
				false,
			},

			{
				fmt.Sprintf(`${chomp("%s")}`, "goodbye\r\nwindows\r\nworld"),
				"goodbye\r\nwindows\r\nworld",
				false,
			},

			{
				fmt.Sprintf(`${chomp("%s")}`, "goodbye\ncruel\nworld\n"),
				"goodbye\ncruel\nworld",
				false,
			},

			{
				fmt.Sprintf(`${chomp("%s")}`, "goodbye\ncruel\nworld\n\n\n\n"),
				"goodbye\ncruel\nworld",
				false,
			},

			{
				fmt.Sprintf(`${chomp("%s")}`, "goodbye\r\nwindows\r\nworld\r\n"),
				"goodbye\r\nwindows\r\nworld",
				false,
			},

			{
				fmt.Sprintf(`${chomp("%s")}`, "goodbye\r\nwindows\r\nworld\r\n\r\n\r\n\r\n"),
				"goodbye\r\nwindows\r\nworld",
				false,
			},
		},
	})
}

func TestInterpolateFuncMap(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// empty input returns empty map
			{
				`${map()}`,
				map[string]interface{}{},
				false,
			},

			// odd args is error
			{
				`${map("odd")}`,
				nil,
				true,
			},

			// two args returns map w/ one k/v
			{
				`${map("hello", "world")}`,
				map[string]interface{}{"hello": "world"},
				false,
			},

			// four args get two k/v
			{
				`${map("hello", "world", "what's", "up?")}`,
				map[string]interface{}{"hello": "world", "what's": "up?"},
				false,
			},

			// map of lists is okay
			{
				`${map("hello", list("world"), "what's", list("up?"))}`,
				map[string]interface{}{
					"hello":  []interface{}{"world"},
					"what's": []interface{}{"up?"},
				},
				false,
			},

			// map of maps is okay
			{
				`${map("hello", map("there", "world"), "what's", map("really", "up?"))}`,
				map[string]interface{}{
					"hello":  map[string]interface{}{"there": "world"},
					"what's": map[string]interface{}{"really": "up?"},
				},
				false,
			},

			// keys have to be strings
			{
				`${map(list("listkey"), "val")}`,
				nil,
				true,
			},

			// types have to match
			{
				`${map("some", "strings", "also", list("lists"))}`,
				nil,
				true,
			},

			// duplicate keys are an error
			{
				`${map("key", "val", "key", "again")}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncCompact(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// empty string within array
			{
				`${compact(split(",", "a,,b"))}`,
				[]interface{}{"a", "b"},
				false,
			},

			// empty string at the end of array
			{
				`${compact(split(",", "a,b,"))}`,
				[]interface{}{"a", "b"},
				false,
			},

			// single empty string
			{
				`${compact(split(",", ""))}`,
				[]interface{}{},
				false,
			},

			// errrors on list of lists
			{
				`${compact(list(list("a"), list("b")))}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncCidrHost(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${cidrhost("192.168.1.0/24", 5)}`,
				"192.168.1.5",
				false,
			},
			{
				`${cidrhost("192.168.1.0/24", -5)}`,
				"192.168.1.251",
				false,
			},
			{
				`${cidrhost("192.168.1.0/24", -256)}`,
				"192.168.1.0",
				false,
			},
			{
				`${cidrhost("192.168.1.0/30", 255)}`,
				nil,
				true, // 255 doesn't fit in two bits
			},
			{
				`${cidrhost("192.168.1.0/30", -255)}`,
				nil,
				true, // 255 doesn't fit in two bits
			},
			{
				`${cidrhost("not-a-cidr", 6)}`,
				nil,
				true, // not a valid CIDR mask
			},
			{
				`${cidrhost("10.256.0.0/8", 6)}`,
				nil,
				true, // can't have an octet >255
			},
		},
	})
}

func TestInterpolateFuncCidrNetmask(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${cidrnetmask("192.168.1.0/24")}`,
				"255.255.255.0",
				false,
			},
			{
				`${cidrnetmask("192.168.1.0/32")}`,
				"255.255.255.255",
				false,
			},
			{
				`${cidrnetmask("0.0.0.0/0")}`,
				"0.0.0.0",
				false,
			},
			{
				// This doesn't really make sense for IPv6 networks
				// but it ought to do something sensible anyway.
				`${cidrnetmask("1::/64")}`,
				"ffff:ffff:ffff:ffff::",
				false,
			},
			{
				`${cidrnetmask("not-a-cidr")}`,
				nil,
				true, // not a valid CIDR mask
			},
			{
				`${cidrnetmask("10.256.0.0/8")}`,
				nil,
				true, // can't have an octet >255
			},
		},
	})
}

func TestInterpolateFuncCidrSubnet(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${cidrsubnet("192.168.2.0/20", 4, 6)}`,
				"192.168.6.0/24",
				false,
			},
			{
				`${cidrsubnet("fe80::/48", 16, 6)}`,
				"fe80:0:0:6::/64",
				false,
			},
			{
				// IPv4 address encoded in IPv6 syntax gets normalized
				`${cidrsubnet("::ffff:192.168.0.0/112", 8, 6)}`,
				"192.168.6.0/24",
				false,
			},
			{
				`${cidrsubnet("192.168.0.0/30", 4, 6)}`,
				nil,
				true, // not enough bits left
			},
			{
				`${cidrsubnet("192.168.0.0/16", 2, 16)}`,
				nil,
				true, // can't encode 16 in 2 bits
			},
			{
				`${cidrsubnet("not-a-cidr", 4, 6)}`,
				nil,
				true, // not a valid CIDR mask
			},
			{
				`${cidrsubnet("10.256.0.0/8", 4, 6)}`,
				nil,
				true, // can't have an octet >255
			},
		},
	})
}

func TestInterpolateFuncCoalesce(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${coalesce("first", "second", "third")}`,
				"first",
				false,
			},
			{
				`${coalesce("", "second", "third")}`,
				"second",
				false,
			},
			{
				`${coalesce("", "", "")}`,
				"",
				false,
			},
			{
				`${coalesce("foo")}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncCoalesceList(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${coalescelist(list("first"), list("second"), list("third"))}`,
				[]interface{}{"first"},
				false,
			},
			{
				`${coalescelist(list(), list("second"), list("third"))}`,
				[]interface{}{"second"},
				false,
			},
			{
				`${coalescelist(list(), list(), list())}`,
				[]interface{}{},
				false,
			},
			{
				`${coalescelist(list("foo"))}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncConcat(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// String + list
			// no longer supported, now returns an error
			{
				`${concat("a", split(",", "b,c"))}`,
				nil,
				true,
			},

			// List + string
			// no longer supported, now returns an error
			{
				`${concat(split(",", "a,b"), "c")}`,
				nil,
				true,
			},

			// Single list
			{
				`${concat(split(",", ",foo,"))}`,
				[]interface{}{"", "foo", ""},
				false,
			},
			{
				`${concat(split(",", "a,b,c"))}`,
				[]interface{}{"a", "b", "c"},
				false,
			},

			// Two lists
			{
				`${concat(split(",", "a,b,c"), split(",", "d,e"))}`,
				[]interface{}{"a", "b", "c", "d", "e"},
				false,
			},
			// Two lists with different separators
			{
				`${concat(split(",", "a,b,c"), split(" ", "d e"))}`,
				[]interface{}{"a", "b", "c", "d", "e"},
				false,
			},

			// More lists
			{
				`${concat(split(",", "a,b"), split(",", "c,d"), split(",", "e,f"), split(",", "0,1"))}`,
				[]interface{}{"a", "b", "c", "d", "e", "f", "0", "1"},
				false,
			},

			// list vars
			{
				`${concat("${var.list}", "${var.list}")}`,
				[]interface{}{"a", "b", "a", "b"},
				false,
			},
			// lists of lists
			{
				`${concat("${var.lists}", "${var.lists}")}`,
				[]interface{}{[]interface{}{"c", "d"}, []interface{}{"c", "d"}},
				false,
			},

			// lists of maps
			{
				`${concat("${var.maps}", "${var.maps}")}`,
				[]interface{}{map[string]interface{}{"key1": "a", "key2": "b"}, map[string]interface{}{"key1": "a", "key2": "b"}},
				false,
			},

			// multiple strings
			// no longer supported, now returns an error
			{
				`${concat("string1", "string2")}`,
				nil,
				true,
			},

			// mismatched types
			{
				`${concat("${var.lists}", "${var.maps}")}`,
				nil,
				true,
			},
		},
		Vars: map[string]ast.Variable{
			"var.list": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type:  ast.TypeString,
						Value: "a",
					},
					{
						Type:  ast.TypeString,
						Value: "b",
					},
				},
			},
			"var.lists": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type: ast.TypeList,
						Value: []ast.Variable{
							{
								Type:  ast.TypeString,
								Value: "c",
							},
							{
								Type:  ast.TypeString,
								Value: "d",
							},
						},
					},
				},
			},
			"var.maps": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type: ast.TypeMap,
						Value: map[string]ast.Variable{
							"key1": {
								Type:  ast.TypeString,
								Value: "a",
							},
							"key2": {
								Type:  ast.TypeString,
								Value: "b",
							},
						},
					},
				},
			},
		},
	})
}

func TestInterpolateFuncContains(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.listOfStrings": interfaceToVariableSwallowError([]string{"notfoo", "stillnotfoo", "bar"}),
			"var.listOfInts":    interfaceToVariableSwallowError([]int{1, 2, 3}),
		},
		Cases: []testFunctionCase{
			{
				`${contains(var.listOfStrings, "bar")}`,
				"true",
				false,
			},

			{
				`${contains(var.listOfStrings, "foo")}`,
				"false",
				false,
			},
			{
				`${contains(var.listOfInts, 1)}`,
				"true",
				false,
			},
			{
				`${contains(var.listOfInts, 10)}`,
				"false",
				false,
			},
			{
				`${contains(var.listOfInts, "2")}`,
				"true",
				false,
			},
		},
	})
}

func TestInterpolateFuncMerge(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// basic merge
			{
				`${merge(map("a", "b"), map("c", "d"))}`,
				map[string]interface{}{"a": "b", "c": "d"},
				false,
			},

			// merge with conflicts is ok, last in wins.
			{
				`${merge(map("a", "b", "c", "X"), map("c", "d"))}`,
				map[string]interface{}{"a": "b", "c": "d"},
				false,
			},

			// merge variadic
			{
				`${merge(map("a", "b"), map("c", "d"), map("e", "f"))}`,
				map[string]interface{}{"a": "b", "c": "d", "e": "f"},
				false,
			},

			// merge with variables
			{
				`${merge(var.maps[0], map("c", "d"))}`,
				map[string]interface{}{"key1": "a", "key2": "b", "c": "d"},
				false,
			},

			// only accept maps
			{
				`${merge(map("a", "b"), list("c", "d"))}`,
				nil,
				true,
			},

			// merge maps of maps
			{
				`${merge(map("a", var.maps[0]), map("b", var.maps[1]))}`,
				map[string]interface{}{
					"b": map[string]interface{}{"key3": "d", "key4": "c"},
					"a": map[string]interface{}{"key1": "a", "key2": "b"},
				},
				false,
			},
			// merge maps of lists
			{
				`${merge(map("a", list("b")), map("c", list("d", "e")))}`,
				map[string]interface{}{"a": []interface{}{"b"}, "c": []interface{}{"d", "e"}},
				false,
			},
			// merge map of various kinds
			{
				`${merge(map("a", var.maps[0]), map("b", list("c", "d")))}`,
				map[string]interface{}{"a": map[string]interface{}{"key1": "a", "key2": "b"}, "b": []interface{}{"c", "d"}},
				false,
			},
		},
		Vars: map[string]ast.Variable{
			"var.maps": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type: ast.TypeMap,
						Value: map[string]ast.Variable{
							"key1": {
								Type:  ast.TypeString,
								Value: "a",
							},
							"key2": {
								Type:  ast.TypeString,
								Value: "b",
							},
						},
					},
					{
						Type: ast.TypeMap,
						Value: map[string]ast.Variable{
							"key3": {
								Type:  ast.TypeString,
								Value: "d",
							},
							"key4": {
								Type:  ast.TypeString,
								Value: "c",
							},
						},
					},
				},
			},
		},
	})

}

func TestInterpolateFuncDirname(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${dirname("/foo/bar/baz")}`,
				filepath.Dir("/foo/bar/baz"),
				false,
			},
		},
	})
}

func TestInterpolateFuncDistinct(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// 3 duplicates
			{
				`${distinct(concat(split(",", "user1,user2,user3"), split(",", "user1,user2,user3")))}`,
				[]interface{}{"user1", "user2", "user3"},
				false,
			},
			// 1 duplicate
			{
				`${distinct(concat(split(",", "user1,user2,user3"), split(",", "user1,user4")))}`,
				[]interface{}{"user1", "user2", "user3", "user4"},
				false,
			},
			// too many args
			{
				`${distinct(concat(split(",", "user1,user2,user3"), split(",", "user1,user4")), "foo")}`,
				nil,
				true,
			},
			// non-flat list is an error
			{
				`${distinct(list(list("a"), list("a")))}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncMatchKeys(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// normal usage
			{
				`${matchkeys(list("a", "b", "c"), list("ref1", "ref2", "ref3"), list("ref2"))}`,
				[]interface{}{"b"},
				false,
			},
			// normal usage 2, check the order
			{
				`${matchkeys(list("a", "b", "c"), list("ref1", "ref2", "ref3"), list("ref2", "ref1"))}`,
				[]interface{}{"a", "b"},
				false,
			},
			// duplicate item in searchset
			{
				`${matchkeys(list("a", "b", "c"), list("ref1", "ref2", "ref3"), list("ref2", "ref2"))}`,
				[]interface{}{"b"},
				false,
			},
			// no matches
			{
				`${matchkeys(list("a", "b", "c"), list("ref1", "ref2", "ref3"), list("ref4"))}`,
				[]interface{}{},
				false,
			},
			// no matches 2
			{
				`${matchkeys(list("a", "b", "c"), list("ref1", "ref2", "ref3"), list())}`,
				[]interface{}{},
				false,
			},
			// zero case
			{
				`${matchkeys(list(), list(), list("nope"))}`,
				[]interface{}{},
				false,
			},
			// complex values
			{
				`${matchkeys(list(list("a", "a")), list("a"), list("a"))}`,
				[]interface{}{[]interface{}{"a", "a"}},
				false,
			},
			// errors
			// different types
			{
				`${matchkeys(list("a"), list(1), list("a"))}`,
				nil,
				true,
			},
			// different types
			{
				`${matchkeys(list("a"), list(list("a"), list("a")), list("a"))}`,
				nil,
				true,
			},
			// lists of different length is an error
			{
				`${matchkeys(list("a"), list("a", "b"), list("a"))}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncFile(t *testing.T) {
    t.Parallel()
	tf, err := os.CreateTemp("", "tf")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	path := strings.ReplaceAll(tf.Name(), `\`, `\\`)
	tf.Write([]byte("foo"))
	tf.Close()
	defer os.Remove(path)

	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				fmt.Sprintf(`${file("%s")}`, path),
				"foo",
				false,
			},

			// Invalid path
			{
				`${file("/i/dont/exist")}`,
				nil,
				true,
			},

			// Too many args
			{
				`${file("foo", "bar")}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncFormat(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${format("hello")}`,
				"hello",
				false,
			},

			{
				`${format("hello %s", "world")}`,
				"hello world",
				false,
			},

			{
				`${format("hello %d", 42)}`,
				"hello 42",
				false,
			},

			{
				`${format("hello %05d", 42)}`,
				"hello 00042",
				false,
			},

			{
				`${format("hello %05d", 12345)}`,
				"hello 12345",
				false,
			},
		},
	})
}

func TestInterpolateFuncFormatList(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// formatlist requires at least one list
			{
				`${formatlist("hello")}`,
				nil,
				true,
			},
			{
				`${formatlist("hello %s", "world")}`,
				nil,
				true,
			},
			// formatlist applies to each list element in turn
			{
				`${formatlist("<%s>", split(",", "A,B"))}`,
				[]interface{}{"<A>", "<B>"},
				false,
			},
			// formatlist repeats scalar elements
			{
				`${join(", ", formatlist("%s=%s", "x", split(",", "A,B,C")))}`,
				"x=A, x=B, x=C",
				false,
			},
			// Multiple lists are walked in parallel
			{
				`${join(", ", formatlist("%s=%s", split(",", "A,B,C"), split(",", "1,2,3")))}`,
				"A=1, B=2, C=3",
				false,
			},
			// Mismatched list lengths generate an error
			{
				`${formatlist("%s=%2s", split(",", "A,B,C,D"), split(",", "1,2,3"))}`,
				nil,
				true,
			},
			// Works with lists of length 1 [GH-2240]
			{
				`${formatlist("%s.id", split(",", "demo-rest-elb"))}`,
				[]interface{}{"demo-rest-elb.id"},
				false,
			},
			// Works with empty lists [GH-7607]
			{
				`${formatlist("%s", var.emptylist)}`,
				[]interface{}{},
				false,
			},
		},
		Vars: map[string]ast.Variable{
			"var.emptylist": {
				Type:  ast.TypeList,
				Value: []ast.Variable{},
			},
		},
	})
}

func TestInterpolateFuncIndex(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.list1": interfaceToVariableSwallowError([]string{"notfoo", "stillnotfoo", "bar"}),
			"var.list2": interfaceToVariableSwallowError([]string{"foo"}),
			"var.list3": interfaceToVariableSwallowError([]string{"foo", "spam", "bar", "eggs"}),
		},
		Cases: []testFunctionCase{
			{
				`${index("test", "")}`,
				nil,
				true,
			},

			{
				`${index(var.list1, "foo")}`,
				nil,
				true,
			},

			{
				`${index(var.list2, "foo")}`,
				"0",
				false,
			},

			{
				`${index(var.list3, "bar")}`,
				"2",
				false,
			},
		},
	})
}

func TestInterpolateFuncIndent(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${indent(4, "Fleas:
Adam
Had'em

E.E. Cummings")}`,
				"Fleas:\n    Adam\n    Had'em\n    \n    E.E. Cummings",
				false,
			},
			{
				`${indent(4, "oneliner")}`,
				"oneliner",
				false,
			},
			{
				`${indent(4, "#!/usr/bin/env bash
date
pwd")}`,
				"#!/usr/bin/env bash\n    date\n    pwd",
				false,
			},
		},
	})
}

func TestInterpolateFuncJoin(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.a_list":        interfaceToVariableSwallowError([]string{"foo"}),
			"var.a_longer_list": interfaceToVariableSwallowError([]string{"foo", "bar", "baz"}),
			"var.list_of_lists": interfaceToVariableSwallowError([]interface{}{[]string{"foo"}, []string{"bar"}, []string{"baz"}}),
		},
		Cases: []testFunctionCase{
			{
				`${join(",")}`,
				nil,
				true,
			},

			{
				`${join(",", var.a_list)}`,
				"foo",
				false,
			},

			{
				`${join(".", var.a_longer_list)}`,
				"foo.bar.baz",
				false,
			},

			{
				`${join(".", var.list_of_lists)}`,
				nil,
				true,
			},
			{
				`${join(".", list(list("nested")))}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncJSONEncode(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"easy": {
				Value: "test",
				Type:  ast.TypeString,
			},
			"hard": {
				Value: " foo \\ \n \t \" bar ",
				Type:  ast.TypeString,
			},
			"list": interfaceToVariableSwallowError([]string{"foo", "bar\tbaz"}),
			"emptylist": {
				Value: []ast.Variable{},
				Type:  ast.TypeList,
			},
			"map": interfaceToVariableSwallowError(map[string]string{
				"foo":     "bar",
				"ba \n z": "q\\x",
			}),
			"emptymap":   interfaceToVariableSwallowError(map[string]string{}),
			"nestedlist": interfaceToVariableSwallowError([][]string{{"foo"}}),
			"nestedmap":  interfaceToVariableSwallowError(map[string][]string{"foo": {"bar"}}),
		},
		Cases: []testFunctionCase{
			{
				`${jsonencode("test")}`,
				`"test"`,
				false,
			},
			{
				`${jsonencode(easy)}`,
				`"test"`,
				false,
			},
			{
				`${jsonencode(hard)}`,
				`" foo \\ \n \t \" bar "`,
				false,
			},
			{
				`${jsonencode("")}`,
				`""`,
				false,
			},
			{
				`${jsonencode()}`,
				nil,
				true,
			},
			{
				`${jsonencode(list)}`,
				`["foo","bar\tbaz"]`,
				false,
			},
			{
				`${jsonencode(emptylist)}`,
				`[]`,
				false,
			},
			{
				`${jsonencode(map)}`,
				`{"ba \n z":"q\\x","foo":"bar"}`,
				false,
			},
			{
				`${jsonencode(emptymap)}`,
				`{}`,
				false,
			},
			{
				`${jsonencode(nestedlist)}`,
				`[["foo"]]`,
				false,
			},
			{
				`${jsonencode(nestedmap)}`,
				`{"foo":["bar"]}`,
				false,
			},
		},
	})
}

func TestInterpolateFuncReplace(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// Regular search and replace
			{
				`${replace("hello", "hel", "bel")}`,
				"bello",
				false,
			},

			// Search string doesn't match
			{
				`${replace("hello", "nope", "bel")}`,
				"hello",
				false,
			},

			// Regular expression
			{
				`${replace("hello", "/l/", "L")}`,
				"heLLo",
				false,
			},

			{
				`${replace("helo", "/(l)/", "$1$1")}`,
				"hello",
				false,
			},

			// Bad regexp
			{
				`${replace("helo", "/(l/", "$1$1")}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncReverse(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.inputlist": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{Type: ast.TypeString, Value: "a"},
					{Type: ast.TypeString, Value: "b"},
					{Type: ast.TypeString, Value: "1"},
					{Type: ast.TypeString, Value: "d"},
				},
			},
			"var.emptylist": {
				Type: ast.TypeList,
				// Intentionally 0-lengthed list
				Value: []ast.Variable{},
			},
		},
		Cases: []testFunctionCase{
			{
				`${reverse(var.inputlist)}`,
				[]interface{}{"d", "1", "b", "a"},
				false,
			},
			{
				`${reverse(var.emptylist)}`,
				[]interface{}{},
				false,
			},
		},
	})
}

func TestInterpolateFuncLength(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// Raw strings
			{
				`${length("")}`,
				"0",
				false,
			},
			{
				`${length("a")}`,
				"1",
				false,
			},
			{
				`${length(" ")}`,
				"1",
				false,
			},
			{
				`${length(" a ,")}`,
				"4",
				false,
			},
			{
				`${length("aaa")}`,
				"3",
				false,
			},

			// Lists
			{
				`${length(split(",", "a"))}`,
				"1",
				false,
			},
			{
				`${length(split(",", "foo,"))}`,
				"2",
				false,
			},
			{
				`${length(split(",", ",foo,"))}`,
				"3",
				false,
			},
			{
				`${length(split(",", "foo,bar"))}`,
				"2",
				false,
			},
			{
				`${length(split(".", "one.two.three.four.five"))}`,
				"5",
				false,
			},
			// Want length 0 if we split an empty string then compact
			{
				`${length(compact(split(",", "")))}`,
				"0",
				false,
			},
			// Works for maps
			{
				`${length(map("k", "v"))}`,
				"1",
				false,
			},
			{
				`${length(map("k1", "v1", "k2", "v2"))}`,
				"2",
				false,
			},
		},
	})
}

func TestInterpolateFuncSignum(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${signum()}`,
				nil,
				true,
			},

			{
				`${signum("")}`,
				nil,
				true,
			},

			{
				`${signum(0)}`,
				"0",
				false,
			},

			{
				`${signum(15)}`,
				"1",
				false,
			},

			{
				`${signum(-29)}`,
				"-1",
				false,
			},
		},
	})
}

func TestInterpolateFuncSlice(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// Negative from index
			{
				`${slice(list("a"), -1, 0)}`,
				nil,
				true,
			},
			// From index > to index
			{
				`${slice(list("a", "b", "c"), 2, 1)}`,
				nil,
				true,
			},
			// To index too large
			{
				`${slice(var.list_of_strings, 1, 4)}`,
				nil,
				true,
			},
			// Empty slice
			{
				`${slice(var.list_of_strings, 1, 1)}`,
				[]interface{}{},
				false,
			},
			{
				`${slice(var.list_of_strings, 1, 2)}`,
				[]interface{}{"b"},
				false,
			},
			{
				`${slice(var.list_of_strings, 0, length(var.list_of_strings) - 1)}`,
				[]interface{}{"a", "b"},
				false,
			},
		},
		Vars: map[string]ast.Variable{
			"var.list_of_strings": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{
						Type:  ast.TypeString,
						Value: "a",
					},
					{
						Type:  ast.TypeString,
						Value: "b",
					},
					{
						Type:  ast.TypeString,
						Value: "c",
					},
				},
			},
		},
	})
}

func TestInterpolateFuncSort(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.strings": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{Type: ast.TypeString, Value: "c"},
					{Type: ast.TypeString, Value: "a"},
					{Type: ast.TypeString, Value: "b"},
				},
			},
			"var.notstrings": {
				Type: ast.TypeList,
				Value: []ast.Variable{
					{Type: ast.TypeList, Value: []ast.Variable{}},
					{Type: ast.TypeString, Value: "b"},
				},
			},
		},
		Cases: []testFunctionCase{
			{
				`${sort(var.strings)}`,
				[]interface{}{"a", "b", "c"},
				false,
			},
			{
				`${sort(var.notstrings)}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncSplit(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${split(",")}`,
				nil,
				true,
			},

			{
				`${split(",", "")}`,
				[]interface{}{""},
				false,
			},

			{
				`${split(",", "foo")}`,
				[]interface{}{"foo"},
				false,
			},

			{
				`${split(",", ",,,")}`,
				[]interface{}{"", "", "", ""},
				false,
			},

			{
				`${split(",", "foo,")}`,
				[]interface{}{"foo", ""},
				false,
			},

			{
				`${split(",", ",foo,")}`,
				[]interface{}{"", "foo", ""},
				false,
			},

			{
				`${split(".", "foo.bar.baz")}`,
				[]interface{}{"foo", "bar", "baz"},
				false,
			},
		},
	})
}

func TestInterpolateFuncLookup(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.foo": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"bar": {
						Type:  ast.TypeString,
						Value: "baz",
					},
				},
			},
			"var.map_of_lists": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"bar": {
						Type: ast.TypeList,
						Value: []ast.Variable{
							{
								Type:  ast.TypeString,
								Value: "baz",
							},
						},
					},
				},
			},
		},
		Cases: []testFunctionCase{
			{
				`${lookup(var.foo, "bar")}`,
				"baz",
				false,
			},

			// Invalid key
			{
				`${lookup(var.foo, "baz")}`,
				nil,
				true,
			},

			// Supplied default with valid key
			{
				`${lookup(var.foo, "bar", "")}`,
				"baz",
				false,
			},

			// Supplied default with invalid key
			{
				`${lookup(var.foo, "zip", "")}`,
				"",
				false,
			},

			// Too many args
			{
				`${lookup(var.foo, "bar", "", "abc")}`,
				nil,
				true,
			},

			// Cannot lookup into map of lists
			{
				`${lookup(var.map_of_lists, "bar")}`,
				nil,
				true,
			},

			// Non-empty default
			{
				`${lookup(var.foo, "zap", "xyz")}`,
				"xyz",
				false,
			},
		},
	})
}

func TestInterpolateFuncKeys(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.foo": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"bar": {
						Value: "baz",
						Type:  ast.TypeString,
					},
					"qux": {
						Value: "quack",
						Type:  ast.TypeString,
					},
				},
			},
			"var.str": {
				Value: "astring",
				Type:  ast.TypeString,
			},
		},
		Cases: []testFunctionCase{
			{
				`${keys(var.foo)}`,
				[]interface{}{"bar", "qux"},
				false,
			},

			// Invalid key
			{
				`${keys(var.not)}`,
				nil,
				true,
			},

			// Too many args
			{
				`${keys(var.foo, "bar")}`,
				nil,
				true,
			},

			// Not a map
			{
				`${keys(var.str)}`,
				nil,
				true,
			},
		},
	})
}

// Confirm that keys return in sorted order, and values return in the order of
// their sorted keys.
func TestInterpolateFuncKeyValOrder(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.foo": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"D": {
						Value: "2",
						Type:  ast.TypeString,
					},
					"C": {
						Value: "Y",
						Type:  ast.TypeString,
					},
					"A": {
						Value: "X",
						Type:  ast.TypeString,
					},
					"10": {
						Value: "Z",
						Type:  ast.TypeString,
					},
					"1": {
						Value: "4",
						Type:  ast.TypeString,
					},
					"3": {
						Value: "W",
						Type:  ast.TypeString,
					},
				},
			},
		},
		Cases: []testFunctionCase{
			{
				`${keys(var.foo)}`,
				[]interface{}{"1", "10", "3", "A", "C", "D"},
				false,
			},

			{
				`${values(var.foo)}`,
				[]interface{}{"4", "Z", "W", "X", "Y", "2"},
				false,
			},
		},
	})
}

func TestInterpolateFuncValues(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.foo": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"bar": {
						Value: "quack",
						Type:  ast.TypeString,
					},
					"qux": {
						Value: "baz",
						Type:  ast.TypeString,
					},
				},
			},
			"var.str": {
				Value: "astring",
				Type:  ast.TypeString,
			},
		},
		Cases: []testFunctionCase{
			{
				`${values(var.foo)}`,
				[]interface{}{"quack", "baz"},
				false,
			},

			// Invalid key
			{
				`${values(var.not)}`,
				nil,
				true,
			},

			// Too many args
			{
				`${values(var.foo, "bar")}`,
				nil,
				true,
			},

			// Not a map
			{
				`${values(var.str)}`,
				nil,
				true,
			},

			// Map of lists
			{
				`${values(map("one", list()))}`,
				nil,
				true,
			},
		},
	})
}

func interfaceToVariableSwallowError(input interface{}) ast.Variable {
	variable, _ := hil.InterfaceToVariable(input)
	return variable
}

func TestInterpolateFuncElement(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.a_list":        interfaceToVariableSwallowError([]string{"foo", "baz"}),
			"var.a_short_list":  interfaceToVariableSwallowError([]string{"foo"}),
			"var.empty_list":    interfaceToVariableSwallowError([]interface{}{}),
			"var.a_nested_list": interfaceToVariableSwallowError([]interface{}{[]string{"foo"}, []string{"baz"}}),
		},
		Cases: []testFunctionCase{
			{
				`${element(var.a_list, "1")}`,
				"baz",
				false,
			},

			{
				`${element(var.a_short_list, "0")}`,
				"foo",
				false,
			},

			// Invalid index should wrap vs. out-of-bounds
			{
				`${element(var.a_list, "2")}`,
				"foo",
				false,
			},

			// Negative number should fail
			{
				`${element(var.a_short_list, "-1")}`,
				nil,
				true,
			},

			// Empty list should fail
			{
				`${element(var.empty_list, 0)}`,
				nil,
				true,
			},

			// Too many args
			{
				`${element(var.a_list, "0", "2")}`,
				nil,
				true,
			},

			// Only works on single-level lists
			{
				`${element(var.a_nested_list, "0")}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncChunklist(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// normal usage
			{
				`${chunklist(list("a", "b", "c"), 1)}`,
				[]interface{}{
					[]interface{}{"a"},
					[]interface{}{"b"},
					[]interface{}{"c"},
				},
				false,
			},
			// `size` is pair and the list has an impair number of items
			{
				`${chunklist(list("a", "b", "c"), 2)}`,
				[]interface{}{
					[]interface{}{"a", "b"},
					[]interface{}{"c"},
				},
				false,
			},
			// list made of the same list, since size is 0
			{
				`${chunklist(list("a", "b", "c"), 0)}`,
				[]interface{}{[]interface{}{"a", "b", "c"}},
				false,
			},
			// negative size of chunks
			{
				`${chunklist(list("a", "b", "c"), -1)}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncBasename(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${basename("/foo/bar/baz")}`,
				"baz",
				false,
			},
		},
	})
}

func TestInterpolateFuncBase64Encode(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// Regular base64 encoding
			{
				`${base64encode("abc123!?$*&()'-=@~")}`,
				"YWJjMTIzIT8kKiYoKSctPUB+",
				false,
			},
		},
	})
}

func TestInterpolateFuncBase64Decode(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// Regular base64 decoding
			{
				`${base64decode("YWJjMTIzIT8kKiYoKSctPUB+")}`,
				"abc123!?$*&()'-=@~",
				false,
			},

			// Invalid base64 data decoding
			{
				`${base64decode("this-is-an-invalid-base64-data")}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncLower(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${lower("HELLO")}`,
				"hello",
				false,
			},

			{
				`${lower("")}`,
				"",
				false,
			},

			{
				`${lower()}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncUpper(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${upper("hello")}`,
				"HELLO",
				false,
			},

			{
				`${upper("")}`,
				"",
				false,
			},

			{
				`${upper()}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncSha1(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${sha1("test")}`,
				"a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
				false,
			},
		},
	})
}

func TestInterpolateFuncSha256(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{ // hexadecimal representation of sha256 sum
				`${sha256("test")}`,
				"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
				false,
			},
		},
	})
}

func TestInterpolateFuncSha512(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${sha512("test")}`,
				"ee26b0dd4af7e749aa1a8ee3c10ae9923f618980772e473f8819a5d4940e0db27ac185f8a0e1d5f84f88bc887fd67b143732c304cc5fa9ad8e6f57f50028a8ff",
				false,
			},
		},
	})
}

func TestInterpolateFuncTitle(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${title("hello")}`,
				"Hello",
				false,
			},

			{
				`${title("hello world")}`,
				"Hello World",
				false,
			},

			{
				`${title("")}`,
				"",
				false,
			},

			{
				`${title()}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncTrimSpace(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${trimspace(" test ")}`,
				"test",
				false,
			},
		},
	})
}

func TestInterpolateFuncBase64Gzip(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${base64gzip("test")}`,
				"H4sIAAAAAAAA/ypJLS4BAAAA//8BAAD//wx+f9gEAAAA",
				false,
			},
		},
	})
}

func TestInterpolateFuncBase64Sha256(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${base64sha256("test")}`,
				"n4bQgYhMfWWaL+qgxVrQFaO/TxsrC4Is0V1sFbDwCgg=",
				false,
			},
			{ // This will differ because we're base64-encoding hex represantiation, not raw bytes
				`${base64encode(sha256("test"))}`,
				"OWY4NmQwODE4ODRjN2Q2NTlhMmZlYWEwYzU1YWQwMTVhM2JmNGYxYjJiMGI4MjJjZDE1ZDZjMTViMGYwMGEwOA==",
				false,
			},
		},
	})
}

func TestInterpolateFuncBase64Sha512(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${base64sha512("test")}`,
				"7iaw3Ur350mqGo7jwQrpkj9hiYB3Lkc/iBml1JQODbJ6wYX4oOHV+E+IvIh/1nsUNzLDBMxfqa2Ob1f1ACio/w==",
				false,
			},
			{ // This will differ because we're base64-encoding hex represantiation, not raw bytes
				`${base64encode(sha512("test"))}`,
				"ZWUyNmIwZGQ0YWY3ZTc0OWFhMWE4ZWUzYzEwYWU5OTIzZjYxODk4MDc3MmU0NzNmODgxOWE1ZDQ5NDBlMGRiMjdhYzE4NWY4YTBlMWQ1Zjg0Zjg4YmM4ODdmZDY3YjE0MzczMmMzMDRjYzVmYTlhZDhlNmY1N2Y1MDAyOGE4ZmY=",
				false,
			},
		},
	})
}

func TestInterpolateFuncMd5(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${md5("tada")}`,
				"ce47d07243bb6eaf5e1322c81baf9bbf",
				false,
			},
			{ // Confirm that we're not trimming any whitespaces
				`${md5(" tada ")}`,
				"aadf191a583e53062de2d02c008141c4",
				false,
			},
			{ // We accept empty string too
				`${md5("")}`,
				"d41d8cd98f00b204e9800998ecf8427e",
				false,
			},
		},
	})
}

func TestInterpolateFuncUUID(t *testing.T) {
    t.Parallel()
	results := make(map[string]bool)

	for i := 0; i < 100; i++ {
		ast, err := hil.Parse("${uuid()}")
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		result, err := hil.Eval(ast, langEvalConfig(nil))
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		if results[result.Value.(string)] {
			t.Fatalf("Got unexpected duplicate uuid: %s", result.Value)
		}

		results[result.Value.(string)] = true
	}
}

func TestInterpolateFuncTimestamp(t *testing.T) {
    t.Parallel()
	currentTime := time.Now().UTC()
	ast, err := hil.Parse("${timestamp()}")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	result, err := hil.Eval(ast, langEvalConfig(nil))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	resultTime, err := time.Parse(time.RFC3339, result.Value.(string))
	if err != nil {
		t.Fatalf("Error parsing timestamp: %s", err)
	}

	if resultTime.Sub(currentTime).Seconds() > 10.0 {
		t.Fatalf("Timestamp Diff too large. Expected: %s\nReceived: %s", currentTime.Format(time.RFC3339), result.Value.(string))
	}
}

func TestInterpolateFuncTimeAdd(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${timeadd("2017-11-22T00:00:00Z", "1s")}`,
				"2017-11-22T00:00:01Z",
				false,
			},
			{
				`${timeadd("2017-11-22T00:00:00Z", "10m1s")}`,
				"2017-11-22T00:10:01Z",
				false,
			},
			{ // also support subtraction
				`${timeadd("2017-11-22T00:00:00Z", "-1h")}`,
				"2017-11-21T23:00:00Z",
				false,
			},
			{ // Invalid format timestamp
				`${timeadd("2017-11-22", "-1h")}`,
				nil,
				true,
			},
			{ // Invalid format duration (day is not supported by ParseDuration)
				`${timeadd("2017-11-22T00:00:00Z", "1d")}`,
				nil,
				true,
			},
		},
	})
}

type testFunctionConfig struct {
	Cases []testFunctionCase
	Vars  map[string]ast.Variable
}

type testFunctionCase struct {
	Input  string
	Result interface{}
	Error  bool
}

func testFunction(t *testing.T, config testFunctionConfig) {
	t.Helper()
	for _, tc := range config.Cases {
		t.Run(tc.Input, func(t *testing.T) {
			ast, err := hil.Parse(tc.Input)
			if err != nil {
				t.Fatalf("unexpected parse error: %s", err)
			}

			result, err := hil.Eval(ast, langEvalConfig(config.Vars))
			if err != nil != tc.Error {
				t.Fatalf("unexpected eval error: %s", err)
			}

			if !reflect.DeepEqual(result.Value, tc.Result) {
				t.Errorf("wrong result\ngiven: %s\ngot:   %#v\nwant:  %#v", tc.Input, result.Value, tc.Result)
			}
		})
	}
}

func TestInterpolateFuncPathExpand(t *testing.T) {
    t.Parallel()
	homePath, err := homedir.Dir()
	if err != nil {
		t.Fatalf("Error getting home directory: %v", err)
	}
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${pathexpand("~/test-file")}`,
				filepath.Join(homePath, "test-file"),
				false,
			},
			{
				`${pathexpand("~/another/test/file")}`,
				filepath.Join(homePath, "another/test/file"),
				false,
			},
			{
				`${pathexpand("/root/file")}`,
				"/root/file",
				false,
			},
			{
				`${pathexpand("/")}`,
				"/",
				false,
			},
			{
				`${pathexpand()}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncSubstr(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${substr("foobar", 0, 0)}`,
				"",
				false,
			},
			{
				`${substr("foobar", 0, -1)}`,
				"foobar",
				false,
			},
			{
				`${substr("foobar", 0, 3)}`,
				"foo",
				false,
			},
			{
				`${substr("foobar", 3, 3)}`,
				"bar",
				false,
			},
			{
				`${substr("foobar", -3, 3)}`,
				"bar",
				false,
			},

			// empty string
			{
				`${substr("", 0, 0)}`,
				"",
				false,
			},

			// invalid offset
			{
				`${substr("", 1, 0)}`,
				nil,
				true,
			},
			{
				`${substr("foo", -4, -1)}`,
				nil,
				true,
			},

			// invalid length
			{
				`${substr("", 0, 1)}`,
				nil,
				true,
			},
			{
				`${substr("", 0, -2)}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncBcrypt(t *testing.T) {
    t.Parallel()
	node, err := hil.Parse(`${bcrypt("test")}`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	result, err := hil.Eval(node, langEvalConfig(nil))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	err = bcrypt.CompareHashAndPassword([]byte(result.Value.(string)), []byte("test"))

	if err != nil {
		t.Fatalf("Error comparing hash and password: %s", err)
	}

	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			//Negative test for more than two parameters
			{
				`${bcrypt("test", 15, 12)}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncFlatten(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			// empty string within array
			{
				`${flatten(split(",", "a,,b"))}`,
				[]interface{}{"a", "", "b"},
				false,
			},

			// typical array
			{
				`${flatten(split(",", "a,b,c"))}`,
				[]interface{}{"a", "b", "c"},
				false,
			},

			// empty array
			{
				`${flatten(split(",", ""))}`,
				[]interface{}{""},
				false,
			},

			// list of lists
			{
				`${flatten(list(list("a"), list("b")))}`,
				[]interface{}{"a", "b"},
				false,
			},
			// list of lists of lists
			{
				`${flatten(list(list("a"), list(list("b","c"))))}`,
				[]interface{}{"a", "b", "c"},
				false,
			},
			// list of strings
			{
				`${flatten(list("a", "b", "c"))}`,
				[]interface{}{"a", "b", "c"},
				false,
			},
		},
	})
}

func TestInterpolateFuncURLEncode(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${urlencode("abc123-_")}`,
				"abc123-_",
				false,
			},
			{
				`${urlencode("foo:bar@localhost?foo=bar&bar=baz")}`,
				"foo%3Abar%40localhost%3Ffoo%3Dbar%26bar%3Dbaz",
				false,
			},
			{
				`${urlencode("mailto:email?subject=this+is+my+subject")}`,
				"mailto%3Aemail%3Fsubject%3Dthis%2Bis%2Bmy%2Bsubject",
				false,
			},
			{
				`${urlencode("foo/bar")}`,
				"foo%2Fbar",
				false,
			},
		},
	})
}

func TestInterpolateFuncTranspose(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.map": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"key1": {
						Type: ast.TypeList,
						Value: []ast.Variable{
							{Type: ast.TypeString, Value: "a"},
							{Type: ast.TypeString, Value: "b"},
						},
					},
					"key2": {
						Type: ast.TypeList,
						Value: []ast.Variable{
							{Type: ast.TypeString, Value: "a"},
							{Type: ast.TypeString, Value: "b"},
							{Type: ast.TypeString, Value: "c"},
						},
					},
					"key3": {
						Type: ast.TypeList,
						Value: []ast.Variable{
							{Type: ast.TypeString, Value: "c"},
						},
					},
					"key4": {
						Type:  ast.TypeList,
						Value: []ast.Variable{},
					},
				}},
			"var.badmap": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"key1": {
						Type: ast.TypeList,
						Value: []ast.Variable{
							{Type: ast.TypeList, Value: []ast.Variable{}},
							{Type: ast.TypeList, Value: []ast.Variable{}},
						},
					},
				}},
			"var.worsemap": {
				Type: ast.TypeMap,
				Value: map[string]ast.Variable{
					"key1": {
						Type:  ast.TypeString,
						Value: "not-a-list",
					},
				}},
		},
		Cases: []testFunctionCase{
			{
				`${transpose(var.map)}`,
				map[string]interface{}{
					"a": []interface{}{"key1", "key2"},
					"b": []interface{}{"key1", "key2"},
					"c": []interface{}{"key2", "key3"},
				},
				false,
			},
			{
				`${transpose(var.badmap)}`,
				nil,
				true,
			},
			{
				`${transpose(var.worsemap)}`,
				nil,
				true,
			},
		},
	})
}

func TestInterpolateFuncAbs(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Cases: []testFunctionCase{
			{
				`${abs()}`,
				nil,
				true,
			},
			{
				`${abs("")}`,
				nil,
				true,
			},
			{
				`${abs(0)}`,
				"0",
				false,
			},
			{
				`${abs(1)}`,
				"1",
				false,
			},
			{
				`${abs(-1)}`,
				"1",
				false,
			},
			{
				`${abs(1.0)}`,
				"1",
				false,
			},
			{
				`${abs(-1.0)}`,
				"1",
				false,
			},
			{
				`${abs(-3.14)}`,
				"3.14",
				false,
			},
			{
				`${abs(-42.001)}`,
				"42.001",
				false,
			},
		},
	})
}

func TestInterpolateFuncRsaDecrypt(t *testing.T) {
    t.Parallel()
	testFunction(t, testFunctionConfig{
		Vars: map[string]ast.Variable{
			"var.cipher_base64": {
				Type:  ast.TypeString,
				Value: "eczGaDhXDbOFRZGhjx2etVzWbRqWDlmq0bvNt284JHVbwCgObiuyX9uV0LSAMY707IEgMkExJqXmsB4OWKxvB7epRB9G/3+F+pcrQpODlDuL9oDUAsa65zEpYF0Wbn7Oh7nrMQncyUPpyr9WUlALl0gRWytOA23S+y5joa4M34KFpawFgoqTu/2EEH4Xl1zo+0fy73fEto+nfkUY+meuyGZ1nUx/+DljP7ZqxHBFSlLODmtuTMdswUbHbXbWneW51D7Jm7xB8nSdiA2JQNK5+Sg5x8aNfgvFTt/m2w2+qpsyFa5Wjeu6fZmXSl840CA07aXbk9vN4I81WmJyblD/ZA==",
			},
			"var.private_key": {
				Type: ast.TypeString,
				Value: `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAgUElV5mwqkloIrM8ZNZ72gSCcnSJt7+/Usa5G+D15YQUAdf9
c1zEekTfHgDP+04nw/uFNFaE5v1RbHaPxhZYVg5ZErNCa/hzn+x10xzcepeS3KPV
Xcxae4MR0BEegvqZqJzN9loXsNL/c3H/B+2Gle3hTxjlWFb3F5qLgR+4Mf4ruhER
1v6eHQa/nchi03MBpT4UeJ7MrL92hTJYLdpSyCqmr8yjxkKJDVC2uRrr+sTSxfh7
r6v24u/vp/QTmBIAlNPgadVAZw17iNNb7vjV7Gwl/5gHXonCUKURaV++dBNLrHIZ
pqcAM8wHRph8mD1EfL9hsz77pHewxolBATV+7QIDAQABAoIBAC1rK+kFW3vrAYm3
+8/fQnQQw5nec4o6+crng6JVQXLeH32qXShNf8kLLG/Jj0vaYcTPPDZw9JCKkTMQ
0mKj9XR/5DLbBMsV6eNXXuvJJ3x4iKW5eD9WkLD4FKlNarBRyO7j8sfPTqXW7uat
NxWdFH7YsSRvNh/9pyQHLWA5OituidMrYbc3EUx8B1GPNyJ9W8Q8znNYLfwYOjU4
Wv1SLE6qGQQH9Q0WzA2WUf8jklCYyMYTIywAjGb8kbAJlKhmj2t2Igjmqtwt1PYc
pGlqbtQBDUiWXt5S4YX/1maIQ/49yeNUajjpbJiH3DbhJbHwFTzP3pZ9P9GHOzlG
kYR+wSECgYEAw/Xida8kSv8n86V3qSY/I+fYQ5V+jDtXIE+JhRnS8xzbOzz3v0WS
Oo5H+o4nJx5eL3Ghb3Gcm0Jn46dHrxinHbm+3RjXv/X6tlbxIYjRSQfHOTSMCTvd
qcliF5vC6RCLXuc7R+IWR1Ky6eDEZGtrvt3DyeYABsp9fRUFR/6NluUCgYEAqNsw
1aSl7WJa27F0DoJdlU9LWerpXcazlJcIdOz/S9QDmSK3RDQTdqfTxRmrxiYI9LEs
mkOkvzlnnOBMpnZ3ZOU5qIRfprecRIi37KDAOHWGnlC0EWGgl46YLb7/jXiWf0AG
Y+DfJJNd9i6TbIDWu8254/erAS6bKMhW/3q7f2kCgYAZ7Id/BiKJAWRpqTRBXlvw
BhXoKvjI2HjYP21z/EyZ+PFPzur/lNaZhIUlMnUfibbwE9pFggQzzf8scM7c7Sf+
mLoVSdoQ/Rujz7CqvQzi2nKSsM7t0curUIb3lJWee5/UeEaxZcmIufoNUrzohAWH
BJOIPDM4ssUTLRq7wYM9uQKBgHCBau5OP8gE6mjKuXsZXWUoahpFLKwwwmJUp2vQ
pOFPJ/6WZOlqkTVT6QPAcPUbTohKrF80hsZqZyDdSfT3peFx4ZLocBrS56m6NmHR
UYHMvJ8rQm76T1fryHVidz85g3zRmfBeWg8yqT5oFg4LYgfLsPm1gRjOhs8LfPvI
OLlRAoGBAIZ5Uv4Z3s8O7WKXXUe/lq6j7vfiVkR1NW/Z/WLKXZpnmvJ7FgxN4e56
RXT7GwNQHIY8eDjDnsHxzrxd+raOxOZeKcMHj3XyjCX3NHfTscnsBPAGYpY/Wxzh
T8UYnFu6RzkixElTf2rseEav7rkdKkI3LAeIZy7B0HulKKsmqVQ7
-----END RSA PRIVATE KEY-----
`,
			},
			"var.wrong_private_key": {
				Type: ast.TypeString,
				Value: `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAlrCgnEVgmNKCq7KPc+zUU5IrxPu1ClMNJS7RTsTPEkbwe5SB
p+6V6WtCbD/X/lDRRGbOENChh1Phulb7lViqgrdpHydgsrKoS5ah3DfSIxLFLE00
9Yo4TCYwgw6+s59j16ZAFVinaQ9l6Kmrb2ll136hMrz8QKh+qw+onOLd38WFgm+W
ZtUqSXf2LANzfzzy4OWFNyFqKaCAolSkPdTS9Nz+svtScvp002DQp8OdP1AgPO+l
o5N3M38Fftapwg0pCtJ5Zq0NRWIXEonXiTEMA6zy3gEZVOmDxoIFUWnmrqlMJLFy
5S6LDrHSdqJhCxDK6WRZj43X9j8spktk3eGhMwIDAQABAoIBAAem8ID/BOi9x+Tw
LFi2rhGQWqimH4tmrEQ3HGnjlKBY+d1MrUjZ1MMFr1nP5CgF8pqGnfA8p/c3Sz8r
K5tp5T6+EZiDZ2WrrOApxg5ox0MAsQKO6SGO40z6o3wEQ6rbbTaGOrraxaWQIpyu
AQanU4Sd6ZGqByVBaS1GnklZO+shCHqw73b7g1cpLEmFzcYnKHYHlUUIsstMe8E1
BaCY0CH7JbWBjcbiTnBVwIRZuu+EjGiQuhTilYL2OWqoMVg1WU0L2IFpR8lkf/2W
SBx5J6xhwbBGASOpM+qidiN580GdPzGhWYSqKGroHEzBm6xPSmV1tadNA26WFG4p
pthLiAECgYEA5BsPRpNYJAQLu5B0N7mj9eEp0HABVEgL/MpwiImjaKdAwp78HM64
IuPvJxs7r+xESiIz4JyjR8zrQjYOCKJsARYkmNlEuAz0SkHabCw1BdEBwUhjUGVB
efoERK6GxfAoNqmSDwsOvHFOtsmDIlbHmg7G2rUxNVpeou415BSB0B8CgYEAqR4J
YHKk2Ibr9rU+rBU33TcdTGw0aAkFNAVeqM9j0haWuFXmV3RArgoy09lH+2Ha6z/g
fTX2xSDAWV7QUlLOlBRIhurPAo2jO2yCrGHPZcWiugstrR2hTTInigaSnCmK3i7F
6sYmL3S7K01IcVNxSlWvGijtClT92Cl2WUCTfG0CgYAiEjyk4QtQTd5mxLvnOu5X
oqs5PBGmwiAwQRiv/EcRMbJFn7Oupd3xMDSflbzDmTnWDOfMy/jDl8MoH6TW+1PA
kcsjnYhbKWwvz0hN0giVdtOZSDO1ZXpzOrn6fEsbM7T9/TQY1SD9WrtUKCNTNL0Z
sM1ZC6lu+7GZCpW4HKwLJwKBgQCRT0yxQXBg1/UxwuO5ynV4rx2Oh76z0WRWIXMH
S0MyxdP1SWGkrS/SGtM3cg/GcHtA/V6vV0nUcWK0p6IJyjrTw2XZ/zGluPuTWJYi
9dvVT26Vunshrz7kbH7KuwEICy3V4IyQQHeY+QzFlR70uMS0IVFWAepCoWqHbIDT
CYhwNQKBgGPcLXmjpGtkZvggl0aZr9LsvCTckllSCFSI861kivL/rijdNoCHGxZv
dfDkLTLcz9Gk41rD9Gxn/3sqodnTAc3Z2PxFnzg1Q/u3+x6YAgBwI/g/jE2xutGW
H7CurtMwALQ/n/6LUKFmjRZjqbKX9SO2QSaC3grd6sY9Tu+bZjLe
-----END RSA PRIVATE KEY-----
`,
			},
		},
		Cases: []testFunctionCase{
			// Base-64 encoded cipher decrypts correctly
			{
				`${rsadecrypt(var.cipher_base64, var.private_key)}`,
				"message",
				false,
			},
			// Raw cipher
			{
				`${rsadecrypt(base64decode(var.cipher_base64), var.private_key)}`,
				nil,
				true,
			},
			// Wrong key
			{
				`${rsadecrypt(var.cipher_base64, var.wrong_private_key)}`,
				nil,
				true,
			},
			// Bad key
			{
				`${rsadecrypt(var.cipher_base64, "bad key")}`,
				nil,
				true,
			},
			// Empty key
			{
				`${rsadecrypt(var.cipher_base64, "")}`,
				nil,
				true,
			},
			// Bad cipher
			{
				`${rsadecrypt("bad cipher", var.private_key)}`,
				nil,
				true,
			},
			// Bad base64-encoded cipher
			{
				`${rsadecrypt(base64encode("bad cipher"), var.private_key)}`,
				nil,
				true,
			},
			// Empty cipher
			{
				`${rsadecrypt("", var.private_key)}`,
				nil,
				true,
			},
			// Too many arguments
			{
				`${rsadecrypt("", "", "")}`,
				nil,
				true,
			},
			// One argument
			{
				`${rsadecrypt("")}`,
				nil,
				true,
			},
			// No arguments
			{
				`${rsadecrypt()}`,
				nil,
				true,
			},
		},
	})
}
