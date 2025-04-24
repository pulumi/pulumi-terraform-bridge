package sdkv2

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

// Interface assertions to ensure v2ResourceConfig implements the required interfaces
var (
	// Ensure v2ResourceConfig struct implements shim.ResourceConfig
	_ shim.ResourceConfig = v2ResourceConfig{}

	// Ensure *v2ResourceConfig pointer implements shim.ResourceConfigWithGetterForRawConfigMap
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

func (c v2ResourceConfig) GetRawConfigMapWithUnknown() (map[string]any, bool, error) {
	var containsUnknowns bool
	jsonConfigMap := map[string]any{}
	ctyValue := c.tf.CtyValue
	if !ctyValue.IsWhollyKnown() {
		containsUnknowns = true
	}
	configJSONMessage, err := valueshim.FromHCtyValue(ctyValue).Marshal()
	if err != nil {
		return nil, containsUnknowns, fmt.Errorf("error marshaling into raw JSON message: %v", err) // TODO: add err
	}

	err = json.Unmarshal(configJSONMessage, &jsonConfigMap)
	if err != nil {
		return nil, containsUnknowns, err
	}
	return jsonConfigMap, containsUnknowns, nil
}
