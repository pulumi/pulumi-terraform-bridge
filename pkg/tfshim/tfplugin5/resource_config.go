package tfplugin5

import (
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"
)

type resourceConfig map[string]interface{}

func (c resourceConfig) IsSet(k string) bool {
	_, ok := c[k]
	return ok
}

func (c resourceConfig) marshal(ty cty.Type) ([]byte, error) {
	val, err := goToCty(c, ty)
	if err != nil {
		return nil, err
	}
	return msgpack.Marshal(val, ty)
}
