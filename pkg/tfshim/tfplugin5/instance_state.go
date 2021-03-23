package tfplugin5

import (
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.InstanceState((*instanceState)(nil))

type instanceState struct {
	resourceType string
	id           string

	object map[string]interface{}
	meta   map[string]interface{}
}

func (s *instanceState) Type() string {
	return s.resourceType
}

func (s *instanceState) ID() string {
	return s.id
}

func (s *instanceState) Object(sch shim.SchemaMap) (map[string]interface{}, error) {
	return s.object, nil
}

func (s *instanceState) Meta() map[string]interface{} {
	return s.meta
}

func (s *instanceState) getObject() map[string]interface{} {
	if s == nil {
		return nil
	}
	return s.object
}

func (s *instanceState) marshal(ty cty.Type) ([]byte, error) {
	val, err := goToCty(s.getObject(), ty)
	if err != nil {
		return nil, err
	}
	return msgpack.Marshal(val, ty)
}
