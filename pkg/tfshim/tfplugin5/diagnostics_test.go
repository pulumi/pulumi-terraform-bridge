package tfplugin5

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/tfplugin5/proto"
)

var warningsOnly = []*proto.Diagnostic{
	{Severity: proto.Diagnostic_WARNING, Summary: "warning 1"},
	{Severity: proto.Diagnostic_WARNING, Summary: "warning 2"},
}

var errorsOnly = []*proto.Diagnostic{
	{Severity: proto.Diagnostic_ERROR, Summary: "error 1"},
	{Severity: proto.Diagnostic_ERROR, Summary: "error 2"},
}

var mixed = append(append([]*proto.Diagnostic{}, warningsOnly...), errorsOnly...)

func TestWarningsAndErrors(t *testing.T) {
	warnings, errors := unmarshalWarningsAndErrors(warningsOnly)
	assert.Equal(t, []string{"warning 1", "warning 2"}, warnings)
	assert.Empty(t, errors)

	warnings, errors = unmarshalWarningsAndErrors(errorsOnly)
	assert.Empty(t, warnings)
	assert.Equal(t, []error{fmt.Errorf("error 1"), fmt.Errorf("error 2")}, errors)

	warnings, errors = unmarshalWarningsAndErrors(mixed)
	assert.Equal(t, []string{"warning 1", "warning 2"}, warnings)
	assert.Equal(t, []error{fmt.Errorf("error 1"), fmt.Errorf("error 2")}, errors)
}

func TestErrors(t *testing.T) {
	err := unmarshalErrors(warningsOnly)
	assert.NoError(t, err)

	err = unmarshalErrors(errorsOnly)
	assert.Equal(t, multierror.Append(nil, fmt.Errorf("error 1"), fmt.Errorf("error 2")), err)

	err = unmarshalErrors(mixed)
	assert.Equal(t, multierror.Append(nil, fmt.Errorf("error 1"), fmt.Errorf("error 2")), err)
}
