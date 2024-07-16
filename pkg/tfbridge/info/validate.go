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

package info

import (
	"context"
	"errors"
	"fmt"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

// Validate [Provider].
//
// Validate is automatically called as part of `make tfgen`.
func (p *Provider) Validate(context.Context) error {
	c := new(infoCheck)

	res := p.P.ResourcesMap()
	for tk, schema := range p.Resources {
		tf, ok := res.GetOk(tk)
		if !ok {
			// This is checked elsewhere.
			continue
		}
		c.checkResource(tk, tf.Schema(), schema.Fields)
	}

	return c.errorOrNil()
}

func (c *infoCheck) errorOrNil() error {
	errs := make([]error, len(c.errors))
	for i, e := range c.errors {
		errs[i] = e
	}
	return errors.Join(errs...)
}

// During tfgen time, we validate that providers are correctly configured.
//
// That means that we error if non-existent fields are specified or if overrides have no effect.

type checkError struct {
	tfToken string
	path    walk.SchemaPath
	err     error
}

func (e checkError) Unwrap() error { return e.err }

func (e checkError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.tfToken, e.path, e.err.Error())
}

type infoCheck struct {
	errors []checkError

	finishError func(e *checkError)
}

func (c *infoCheck) error(path walk.SchemaPath, err error) {
	e := checkError{
		path: path,
		err:  err,
	}
	c.finishError(&e)
	c.errors = append(c.errors, e)
}

var (
	errNoCorrespondingField           = fmt.Errorf("overriding non-existent field")
	errNoElemToOverride               = fmt.Errorf("overriding non-existent elem")
	errCannotSpecifyFieldsOnListOrSet = fmt.Errorf("cannot specify .Fields on a List[T] or Set[T] type")
	errCannotSpecifyNameOnElem        = fmt.Errorf("cannot specify .Name on a List[T], Map[T] or Set[T] type")
	errCannotSetMaxItemsOne           = fmt.Errorf("cannot specify .MaxItemsOne on a scalar type")
	errElemForObject                  = fmt.Errorf("cannot set .Elem on a singly nested object block")
)

func (c *infoCheck) checkResource(tfToken string, schema shim.SchemaMap, info map[string]*Schema) {
	c.finishError = func(e *checkError) {
		e.tfToken = tfToken
	}
	defer func() {
		c.finishError = nil
	}()
	c.checkFields(walk.NewSchemaPath(), schema, info)
}

func (c *infoCheck) checkProperty(path walk.SchemaPath, tfs shim.Schema, ps *Schema) {
	// If there is no override, then there were no mistakes.
	if ps == nil {
		return
	}

	// s.Elem() case 2
	//
	// `path` represents a singly-nested Terraform block.
	//
	// Conceptually `path` and `path.Elem` represent the same object.
	//
	// To prevent confusion, users are barred from specifying information on
	// the associated Elem. All information should be specified directly on
	// this SchemaInfo.
	if obj, ok := util.CastToTypeObject(tfs); ok {
		if ps.Elem != nil {
			c.error(path, errElemForObject)
		}

		if ps.MaxItemsOne != nil {
			c.error(path.Element(), errCannotSetMaxItemsOne)
		}

		// Now check sub-fields
		c.checkFields(path, obj, ps.Fields)

		return
	}

	if len(ps.Fields) > 0 {
		c.error(path, errCannotSpecifyFieldsOnListOrSet)
	}

	switch elem := tfs.Elem().(type) {

	// s.Elem() case 1
	//
	// There is a simple element type here, so we just recursively validate that.
	case shim.Schema:
		c.checkElem(path.Element(), elem, ps.Elem)

	// Either `path` represents an object nested under a list or set, or `path` is
	// itself an object, depending on the .Type() property.
	case shim.Resource:
		// s.Elem() case 3
		//
		// `path` represents a List[elem] or Set[elem].

		// Check the nested fields
		if ps.Elem != nil {
			c.checkFields(path.Element(), elem.Schema(), ps.Elem.Fields)
		}

	// There is no element for this shim.Schema shape.
	//
	// The only thing the user can do wrong here is to customize the element.
	case nil:
		switch tfs.Type() {
		// s.Elem() case 5
		case shim.TypeMap, shim.TypeList, shim.TypeSet:
			// The type is unknown, but specifying overrides is invalid.
			//
			// We can't check any deeper, so return
			return
		}

		// s.Elem() case 4
		//
		// It is not valid to specify .Elem
		if ps.Elem != nil {
			c.error(path.Element(), errNoElemToOverride)
		}

		if ps.MaxItemsOne != nil {
			c.error(path.Element(), errCannotSetMaxItemsOne)
		}
	}
}

// Check a nested element.
func (c *infoCheck) checkElem(path walk.SchemaPath, tfs shim.Schema, ps *Schema) {
	if ps == nil {
		return
	}
	if ps.Name != "" {
		// If we are an element type, and we are not an object (which has a name),
		// then it doesn't make sense to provide a name since the elem will be
		// accessed by an index.
		c.error(path, errCannotSpecifyNameOnElem)
	}

	c.checkProperty(path, tfs, ps)
}

func (c *infoCheck) checkFields(path walk.SchemaPath, tfs shim.SchemaMap, ps map[string]*Schema) {
	for k, p := range ps {
		elemPath := path.GetAttr(k)
		elemTfs, ok := tfs.GetOk(k)
		if !ok {
			c.error(elemPath, errNoCorrespondingField)
			continue
		}
		c.checkProperty(elemPath, elemTfs, p)
	}
}
