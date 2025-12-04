package adapter

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

//go:embed aws_info.json
var awsInfo []byte

//go:embed archive_info.json
var archiveInfo []byte

func getProviderSchema(providerName string) (*info.Provider, error) {
	// TODO: provider downloading and handling
	var info info.MarshallableProvider
	switch providerName {
	case "aws":
		if err := json.Unmarshal(awsInfo, &info); err != nil {
			return nil, err
		}
	case "archive":
		if err := json.Unmarshal(archiveInfo, &info); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}

	return info.Unmarshal(), nil
}
