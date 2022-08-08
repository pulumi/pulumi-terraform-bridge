package sdkv2

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Resource(v2Resource{})
var _ = shim.ResourceMap(v2ResourceMap{})

type v2Resource struct {
	tf *schema.Resource
}

func NewResource(r *schema.Resource) shim.Resource {
	return v2Resource{r}
}

func (r v2Resource) Schema() shim.SchemaMap {
	return v2SchemaMap(r.tf.Schema)
}

func (r v2Resource) SchemaVersion() int {
	return r.tf.SchemaVersion
}

// nolint: staticcheck
func (r v2Resource) Importer() shim.ImportFunc {
	if r.tf.Importer == nil {
		return nil
	}
	return func(t, id string, meta interface{}) ([]shim.InstanceState, error) {
		data := r.tf.Data(nil)
		data.SetId(id)
		data.SetType(t)

		var v2Results []*schema.ResourceData
		var err error
		switch {
		case r.tf.Importer.State != nil:
			v2Results, err = r.tf.Importer.State(data, meta)
		case r.tf.Importer.StateContext != nil:
			v2Results, err = r.tf.Importer.StateContext(context.TODO(), data, meta)
		}
		if err != nil {
			return nil, err
		}
		results := make([]shim.InstanceState, len(v2Results))
		for i, v := range v2Results {
			s := v.State()
			if s == nil {
				return nil, fmt.Errorf("importer for %s returned a empty resource state. This is always "+
					"the result of a bug in the resource provider - please report this "+
					"as a bug in the Pulumi provider repository.", id)
			}
			if s.Attributes != nil {
				results[i] = v2InstanceState{s, nil}
			}
		}
		return results, nil
	}
}

func (r v2Resource) DeprecationMessage() string {
	return r.tf.DeprecationMessage
}

func (r v2Resource) Timeouts() *shim.ResourceTimeout {
	if r.tf.Timeouts == nil {
		return nil
	}
	return &shim.ResourceTimeout{
		Create:  r.tf.Timeouts.Create,
		Read:    r.tf.Timeouts.Read,
		Update:  r.tf.Timeouts.Update,
		Delete:  r.tf.Timeouts.Delete,
		Default: r.tf.Timeouts.Default,
	}
}

func (r v2Resource) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
	// Read each top-level value out of the object  using a ConfigFieldReader and recursively flatten
	// them into their TF attribute form. The result is our set of TF attributes.
	config := &terraform.ResourceConfig{Raw: object, Config: object}
	attributes := map[string]string{}
	reader := &schema.ConfigFieldReader{Config: config, Schema: r.tf.Schema}
	for k := range r.tf.Schema {
		// Elide nil values.
		if v, ok := object[k]; ok && v == nil {
			continue
		}

		f, err := reader.ReadField([]string{k})
		if err != nil {
			return nil, fmt.Errorf("could not read field %v: %w", k, err)
		}

		flattenValue(attributes, k, f.Value)
	}

	return v2InstanceState{&terraform.InstanceState{
		ID:         id,
		Attributes: attributes,
		Meta:       meta,
	}, nil}, nil
}

func (r v2Resource) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	v2Timeouts := &schema.ResourceTimeout{}
	if err := v2Timeouts.ConfigDecode(r.tf, configFromShim(config)); err != nil {
		return nil, err
	}

	return &shim.ResourceTimeout{
		Create:  v2Timeouts.Create,
		Read:    v2Timeouts.Read,
		Update:  v2Timeouts.Update,
		Delete:  v2Timeouts.Delete,
		Default: v2Timeouts.Default,
	}, nil
}

type v2ResourceMap map[string]*schema.Resource

func (m v2ResourceMap) Len() int {
	return len(m)
}

func (m v2ResourceMap) Get(key string) shim.Resource {
	r, _ := m.GetOk(key)
	return r
}

func (m v2ResourceMap) GetOk(key string) (shim.Resource, bool) {
	if r, ok := m[key]; ok {
		return v2Resource{r}, true
	}
	return nil, false
}

func (m v2ResourceMap) Range(each func(key string, value shim.Resource) bool) {
	for key, value := range m {
		if !each(key, v2Resource{value}) {
			return
		}
	}
}

func (m v2ResourceMap) Set(key string, value shim.Resource) {
	m[key] = value.(v2Resource).tf
}
