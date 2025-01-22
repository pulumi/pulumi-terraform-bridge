package main

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Reference to a Terraform module, for example "terraform-aws-modules/vpc/aws".
type TFModuleRef string

// Version specification for a Terraform module, for example "5.16.0".
type TFModuleVersion string

type ParameterizeArgs struct {
	TFModuleRef     TFModuleRef     `json:"module"`
	TFModuleVersion TFModuleVersion `json:"version"`
}

func (args *ParameterizeArgs) ToParameterizationSpec() *schema.ParameterizationSpec {
	parameter, err := json.MarshalIndent(args, "", "  ")
	contract.AssertNoErrorf(err, "MarshalIndent should not fail")
	return &schema.ParameterizationSpec{
		BaseProvider: schema.BaseProviderSpec{
			Name:    providerName,
			Version: providerVersion,
		},
		Parameter: parameter,
	}
}

func parseParameterizeRequest(request *pulumirpc.ParameterizeRequest) (ParameterizeArgs, error) {
	switch {
	case request.GetArgs() != nil:
		args := request.GetArgs()
		if len(args.Args) != 2 {
			return ParameterizeArgs{}, fmt.Errorf("Expected exactly 2 args")
		}
		return ParameterizeArgs{
			TFModuleRef:     TFModuleRef(args.Args[0]),
			TFModuleVersion: TFModuleVersion(args.Args[1]),
		}, nil
	case request.GetValue() != nil:
		value := request.GetValue()
		var args ParameterizeArgs
		err := json.Unmarshal(value.Value, &args)
		if err != nil {
			return args, fmt.Errorf("ParameterizeRequest.GetValue().Value is not JSON-encoded: %w", err)
		}
		if args.TFModuleRef == "" {
			return args, fmt.Errorf("ParameterizeRequest.GetValue(): module cannot be empty")
		}
		if args.TFModuleVersion == "" {
			return args, fmt.Errorf("ParameterizeRequest.GetValue(): version cannot be empty")
		}
		return args, nil
	default:
		contract.Assertf(false, "malformed pulumirpc.ParameterizeRequest")
		return ParameterizeArgs{}, nil
	}
}
