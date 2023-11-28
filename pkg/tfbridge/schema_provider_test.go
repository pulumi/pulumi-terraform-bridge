package tfbridge

import (
	schemav1 "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	terraformv1 "github.com/hashicorp/terraform-plugin-sdk/terraform"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	terraformv2 "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func schemaMap(m map[string]*schema.Schema) shim.SchemaMap {
	mm := schema.SchemaMap{}
	for k, v := range m {
		mm[k] = v.Shim()
	}
	return mm
}

var (
	testTFProvider   = testprovider.ProviderV1()
	testTFProviderV2 = testprovider.ProviderV2()
)

type shimFactory interface {
	SDKVersion() string
	NewSchema(s *schema.Schema) shim.Schema
	NewSchemaMap(m map[string]*schema.Schema) shim.SchemaMap
	NewResource(r *schema.Resource) shim.Resource
	NewInstanceState(id string) shim.InstanceState
	NewTestProvider() shim.Provider
}

type shimv1Factory int

func (f shimv1Factory) SDKVersion() string {
	return "v1"
}

func (f shimv1Factory) newSchema(m shim.Schema) *schemav1.Schema {
	t := schemav1.TypeInvalid
	switch m.Type() {
	case shim.TypeBool:
		t = schemav1.TypeBool
	case shim.TypeInt:
		t = schemav1.TypeInt
	case shim.TypeFloat:
		t = schemav1.TypeFloat
	case shim.TypeString:
		t = schemav1.TypeString
	case shim.TypeList:
		t = schemav1.TypeList
	case shim.TypeMap:
		t = schemav1.TypeMap
	case shim.TypeSet:
		t = schemav1.TypeSet
	}

	var elem interface{}
	switch e := m.Elem().(type) {
	case shim.Schema:
		elem = f.newSchema(e)
	case shim.Resource:
		elem = f.newResource(e)
	}

	return &schemav1.Schema{
		Type:          t,
		Optional:      m.Optional(),
		Required:      m.Required(),
		Default:       m.Default(),
		DefaultFunc:   schemav1.SchemaDefaultFunc(m.DefaultFunc()),
		Description:   m.Description(),
		Computed:      m.Computed(),
		ForceNew:      m.ForceNew(),
		StateFunc:     schemav1.SchemaStateFunc(m.StateFunc()),
		Elem:          elem,
		MaxItems:      m.MaxItems(),
		MinItems:      m.MinItems(),
		ConflictsWith: m.ConflictsWith(),
		Deprecated:    m.Deprecated(),
		Removed:       m.Removed(),
		Sensitive:     m.Sensitive(),
		ExactlyOneOf:  m.ExactlyOneOf(),
	}
}

func (f shimv1Factory) NewSchema(m *schema.Schema) shim.Schema {
	return shimv1.NewSchema(f.newSchema(m.Shim()))
}

func (f shimv1Factory) newSchemaMap(m shim.SchemaMap) map[string]*schemav1.Schema {
	tf := map[string]*schemav1.Schema{}
	m.Range(func(k string, v shim.Schema) bool {
		tf[k] = f.newSchema(v)
		return true
	})
	return tf
}

func (f shimv1Factory) NewSchemaMap(m map[string]*schema.Schema) shim.SchemaMap {
	tf := map[string]*schemav1.Schema{}
	for k, v := range m {
		tf[k] = f.newSchema(v.Shim())
	}
	return shimv1.NewSchemaMap(tf)
}

func (f shimv1Factory) newResource(r shim.Resource) *schemav1.Resource {
	var timeouts *schemav1.ResourceTimeout
	if t := r.Timeouts(); t != nil {
		timeouts = &schemav1.ResourceTimeout{
			Create:  t.Create,
			Read:    t.Read,
			Update:  t.Update,
			Delete:  t.Delete,
			Default: t.Default,
		}
	}

	return &schemav1.Resource{
		Schema:             f.newSchemaMap(r.Schema()),
		SchemaVersion:      r.SchemaVersion(),
		DeprecationMessage: r.DeprecationMessage(),
		Timeouts:           timeouts,
	}
}

func (f shimv1Factory) NewResource(r *schema.Resource) shim.Resource {
	return shimv1.NewResource(f.newResource(r.Shim()))
}

func (f shimv1Factory) NewInstanceState(id string) shim.InstanceState {
	return shimv1.NewInstanceState(&terraformv1.InstanceState{
		ID: id, Attributes: map[string]string{}, Meta: map[string]interface{}{},
	})
}

func (f shimv1Factory) NewTestProvider() shim.Provider {
	return shimv1.NewProvider(testTFProvider)
}

type shimv2Factory int

func (f shimv2Factory) SDKVersion() string {
	return "v2"
}

func (f shimv2Factory) newSchema(m shim.Schema) *schemav2.Schema {
	t := schemav2.TypeInvalid
	switch m.Type() {
	case shim.TypeBool:
		t = schemav2.TypeBool
	case shim.TypeInt:
		t = schemav2.TypeInt
	case shim.TypeFloat:
		t = schemav2.TypeFloat
	case shim.TypeString:
		t = schemav2.TypeString
	case shim.TypeList:
		t = schemav2.TypeList
	case shim.TypeMap:
		t = schemav2.TypeMap
	case shim.TypeSet:
		t = schemav2.TypeSet
	}

	var elem interface{}
	switch e := m.Elem().(type) {
	case shim.Schema:
		elem = f.newSchema(e)
	case shim.Resource:
		elem = f.newResource(e)
	}

	return &schemav2.Schema{
		Type:          t,
		Optional:      m.Optional(),
		Required:      m.Required(),
		Default:       m.Default(),
		DefaultFunc:   schemav2.SchemaDefaultFunc(m.DefaultFunc()),
		Description:   m.Description(),
		Computed:      m.Computed(),
		ForceNew:      m.ForceNew(),
		StateFunc:     schemav2.SchemaStateFunc(m.StateFunc()),
		Elem:          elem,
		MaxItems:      m.MaxItems(),
		MinItems:      m.MinItems(),
		ConflictsWith: m.ConflictsWith(),
		RequiredWith:  m.RequiredWith(),
		Deprecated:    m.Deprecated(),
		Sensitive:     m.Sensitive(),
		ExactlyOneOf:  m.ExactlyOneOf(),
	}
}

func (f shimv2Factory) NewSchema(m *schema.Schema) shim.Schema {
	return shimv2.NewSchema(f.newSchema(m.Shim()))
}

func (f shimv2Factory) newSchemaMap(m shim.SchemaMap) map[string]*schemav2.Schema {
	tf := map[string]*schemav2.Schema{}
	m.Range(func(k string, v shim.Schema) bool {
		if v.Removed() == "" {
			tf[k] = f.newSchema(v)
		}
		return true
	})
	return tf
}

func (f shimv2Factory) NewSchemaMap(m map[string]*schema.Schema) shim.SchemaMap {
	tf := map[string]*schemav2.Schema{}
	for k, v := range m {
		if v.Removed == "" {
			tf[k] = f.newSchema(v.Shim())
		}
	}
	return shimv2.NewSchemaMap(tf)
}

func (f shimv2Factory) newResource(r shim.Resource) *schemav2.Resource {
	var timeouts *schemav2.ResourceTimeout
	if t := r.Timeouts(); t != nil {
		timeouts = &schemav2.ResourceTimeout{
			Create:  t.Create,
			Read:    t.Read,
			Update:  t.Update,
			Delete:  t.Delete,
			Default: t.Default,
		}
	}

	return &schemav2.Resource{
		Schema:             f.newSchemaMap(r.Schema()),
		SchemaVersion:      r.SchemaVersion(),
		DeprecationMessage: r.DeprecationMessage(),
		Timeouts:           timeouts,
	}
}

func (f shimv2Factory) NewResource(r *schema.Resource) shim.Resource {
	return shimv2.NewResource(f.newResource(r.Shim()))
}

func (f shimv2Factory) NewInstanceState(id string) shim.InstanceState {
	return shimv2.NewInstanceState(&terraformv2.InstanceState{
		ID: id, Attributes: map[string]string{}, Meta: map[string]interface{}{},
	})
}

func (f shimv2Factory) NewTestProvider() shim.Provider {
	return shimv2.NewProvider(testTFProviderV2)
}

var factories = []shimFactory{
	shimv1Factory(0),
	shimv2Factory(0),
}
