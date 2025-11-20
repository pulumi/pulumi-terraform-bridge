package adapter

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func getProviderSchema(providerName string) (*info.Provider, error) {
	// TODO: provider downloading and handling
	if providerName != "aws" {
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}

	data, err := os.ReadFile("aws_info.json")
	if err != nil {
		return nil, err
	}

	var info info.MarshallableProvider
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return info.Unmarshal(), nil
}
