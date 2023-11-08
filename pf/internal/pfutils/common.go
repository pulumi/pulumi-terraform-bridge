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

package pfutils

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func queryProviderMetadata(ctx context.Context, prov provider.Provider) *provider.MetadataResponse {
	provMetadata := provider.MetadataResponse{}
	prov.Metadata(ctx, provider.MetadataRequest{}, &provMetadata)
	return &provMetadata
}

func checkDiagsForErrors(diag diag.Diagnostics) error {
	if diag.HasError() {
		errs := diag.Errors()
		err := fmt.Errorf(
			"Error 1 of %d: %s",
			diag.ErrorsCount(),
			errs[0].Summary(),
		)
		return err
	}
	return nil
}

type entry[T any] struct {
	schema      Schema
	t           T
	diagnostics diag.Diagnostics
}

type collection[T any] map[TypeName]entry[T]

func (c collection[T]) All() []TypeName {
	if c == nil {
		return nil
	}
	var names []TypeName
	for name := range c {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return string(names[i]) < string(names[j])
	})
	return names
}

func (c collection[T]) Has(name TypeName) bool {
	_, ok := c[name]
	return ok
}

func (c collection[T]) Schema(name TypeName) Schema {
	return c[name].schema
}
