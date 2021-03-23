package sdkv1

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Resource(v1Resource{})
var _ = shim.ResourceMap(v1ResourceMap{})

type v1Resource struct {
	tf *schema.Resource
}

func NewResource(r *schema.Resource) shim.Resource {
	return v1Resource{r}
}

func (r v1Resource) Schema() shim.SchemaMap {
	return v1SchemaMap(r.tf.Schema)
}

func (r v1Resource) SchemaVersion() int {
	return r.tf.SchemaVersion
}

func (r v1Resource) Importer() shim.ImportFunc {
	if r.tf.Importer == nil {
		return nil
	}
	return func(t, id string, meta interface{}) ([]shim.InstanceState, error) {
		data := r.tf.Data(nil)
		data.SetId(id)
		data.SetType(t)

		v1Results, err := r.tf.Importer.State(data, meta)
		if err != nil {
			return nil, err
		}
		results := make([]shim.InstanceState, len(v1Results))
		for i, v := range v1Results {
			s := v.State()
			if s == nil {
				return nil, fmt.Errorf("importer for %s returned a empty resource state. This is always "+
					"the result of a bug in the resource provider - please report this "+
					"as a bug in the Pulumi provider repository.", id)
			}
			if s.Attributes != nil {
				results[i] = v1InstanceState{s, nil}
			}
		}
		return results, nil
	}
}

func (r v1Resource) DeprecationMessage() string {
	return r.tf.DeprecationMessage
}

func (r v1Resource) Timeouts() *shim.ResourceTimeout {
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

func (r v1Resource) InstanceState(id string, object, meta map[string]interface{}) (shim.InstanceState, error) {
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

	return v1InstanceState{&terraform.InstanceState{
		ID:         id,
		Attributes: attributes,
		Meta:       meta,
	}, nil}, nil
}

func (r v1Resource) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	v1Timeouts := &schema.ResourceTimeout{}
	if err := v1Timeouts.ConfigDecode(r.tf, configFromShim(config)); err != nil {
		return nil, err
	}

	return &shim.ResourceTimeout{
		Create:  v1Timeouts.Create,
		Read:    v1Timeouts.Read,
		Update:  v1Timeouts.Update,
		Delete:  v1Timeouts.Delete,
		Default: v1Timeouts.Default,
	}, nil
}

type v1ResourceMap map[string]*schema.Resource

func (m v1ResourceMap) Len() int {
	return len(m)
}

func (m v1ResourceMap) Get(key string) shim.Resource {
	r, _ := m.GetOk(key)
	return r
}

func (m v1ResourceMap) GetOk(key string) (shim.Resource, bool) {
	if r, ok := m[key]; ok {
		return v1Resource{r}, true
	}
	return nil, false
}

func (m v1ResourceMap) Range(each func(key string, value shim.Resource) bool) {
	for key, value := range m {
		if !each(key, v1Resource{value}) {
			return
		}
	}
}

func (m v1ResourceMap) Set(key string, value shim.Resource) {
	m[key] = value.(v1Resource).tf
}
