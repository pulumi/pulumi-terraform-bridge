package adapter

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

//go:embed aws_info.json
var awsInfo []byte

func getProviderSchema(providerName string) (*info.Provider, error) {
	// TODO: provider downloading and handling
	if providerName != "aws" {
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}

	var info info.MarshallableProvider
	if err := json.Unmarshal(awsInfo, &info); err != nil {
		return nil, err
	}

	return info.Unmarshal(), nil
}
