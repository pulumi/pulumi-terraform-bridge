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
	errNoCorrespondingField            = fmt.Errorf("overriding non-existent field")
	errNoElemToOverride                = fmt.Errorf("overriding non-existent elem")
	errCannotSpecifyFieldsOnCollection = fmt.Errorf("cannot specify .Fields on a List[T], Set[T] or Map[T] type")
	errCannotSpecifyNameOnElem         = fmt.Errorf("cannot specify .Name on a List[T], Map[T] or Set[T] type")
	errCannotSetMaxItemsOne            = fmt.Errorf("can only specify .MaxItemsOne on List[T] or Set[T] type")
	errFieldsShouldBeOnElem            = fmt.Errorf(".Fields should be .Elem.Fields")
	errPropertyNameCannotBePulumi      = fmt.Errorf("a property should not be named pulumi")
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

	if ps.Name == "pulumi" {
		c.error(path, errPropertyNameCannotBePulumi)
	}

	if t := tfs.Type(); t != shim.TypeSet && t != shim.TypeList && ps.MaxItemsOne != nil {
		c.error(path, errCannotSetMaxItemsOne)
	}

	switch elem := tfs.Elem().(type) {
	// s.Elem() case 1
	//
	// There is a simple element type here, so we just recursively validate that.
	case shim.Schema:
		if len(ps.Fields) > 0 {
			c.error(path, errCannotSpecifyFieldsOnCollection)
		}

		c.checkElem(path.Element(), elem, ps.Elem)

	// Either `path` represents an object nested under a list or set, or `path` is
	// itself an object, depending on the .Type() property.
	case shim.Resource:
		// s.Elem() case 2 and 3
		//
		// `path` represents a List[elem], Set[elem], Map[elem] or Object.
		//
		// The [shim.Schema] representation is unable to distinguish between
		// Map[elem] and Object.

		if tfs.Type() == shim.TypeMap && len(ps.Fields) > 0 {
			// To prevent confusion, users are barred from specifying
			// information on ps directly, they must set .Fields on ps.Elem:
			// ps.Elem.Fields.
			//
			// We need to make this choice (instead of having users set
			// information on .Fields (and forbidding ps.Elem.Fields) because
			// the [shim.Schema] representation is not capable of
			// distinguishing between Map[Object] and Object. SDKv{1,2}
			// providers are not able to express Map[Object], but Plugin
			// Framework based providers are.
			c.error(path, errFieldsShouldBeOnElem)
		}

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
			// The type is unknown, but specifying overrides is valid.
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
