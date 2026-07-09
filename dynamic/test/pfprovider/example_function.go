package main

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

// echoUpperFunction upper-cases its input. It exercises provider-defined functions end
// to end through the dynamic bridge.
type echoUpperFunction struct{}

var _ function.Function = echoUpperFunction{}

func NewEchoUpperFunction() function.Function { return echoUpperFunction{} }

func (echoUpperFunction) Metadata(
	_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse,
) {
	resp.Name = "echo_upper"
}

func (echoUpperFunction) Definition(
	_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse,
) {
	resp.Definition = function.Definition{
		Summary: "Returns the input string upper-cased.",
		Parameters: []function.Parameter{
			function.StringParameter{Name: "input"},
		},
		Return: function.StringReturn{},
	}
}

func (echoUpperFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var input string
	resp.Error = req.Arguments.Get(ctx, &input)
	if resp.Error != nil {
		return
	}
	resp.Error = resp.Result.Set(ctx, strings.ToUpper(input))
}
