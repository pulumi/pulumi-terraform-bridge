package sdkv2

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

var (
	_ shim.ResourceConfig                          = v2ResourceConfig{}
	_ shim.ResourceConfigWithGetterForRawConfigMap = (*v2ResourceConfig)(nil)
)

type v2ResourceConfig struct {
	tf *terraform.ResourceConfig
}

func (c v2ResourceConfig) IsSet(key string) bool {
	if c.tf == nil {
		return false
	}

	if c.tf.IsComputed(key) {
		return true
	}

	if _, ok := c.tf.Get(key); ok {
		return true
	}

	return false
}

func (c v2ResourceConfig) GetRawConfigMap() (map[string]any, error) {
	jsonConfigMap := map[string]any{}
	ctyValue := c.tf.CtyValue
	if !ctyValue.IsWhollyKnown() {
		msg := "ConfigMap contains unknowns"
		return nil, fmt.Errorf("%s", msg)
	}
	configJSONMessage, err := valueshim.FromHCtyValue(ctyValue).Marshal()
	if err != nil {
		return nil, fmt.Errorf("error marshaling into raw JSON message: %v", err)
	}

	err = json.Unmarshal(configJSONMessage, &jsonConfigMap)
	if err != nil {
		return nil, err
	}
	return jsonConfigMap, nil
}
