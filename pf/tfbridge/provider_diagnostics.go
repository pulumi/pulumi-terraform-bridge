// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridge

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
)

func (p *provider) processDiagnostics(diagnostics []*tfprotov6.Diagnostic) error {
	// Format and flush to diagSink.
	if p.diagSink != nil {
		for _, d := range diagnostics {
			p.logDiagnostic(d)
		}
	}

	// Check for errors and return non-nil if there is an error.
	for _, d := range diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			prefix := ""
			if d.Attribute != nil {
				prefix = fmt.Sprintf("[%s] ", d.Attribute.String())
			}
			summary := tfbridge.ReplaceConfigProperties(d.Summary, p.info.Name, p.info.Config, p.info.P.Schema())
			detail := tfbridge.ReplaceConfigProperties(d.Detail, p.info.Name, p.info.Config, p.info.P.Schema())
			return fmt.Errorf("%s%s: %s", prefix, summary, detail)
		}
	}

	return nil
}

func (p *provider) logDiagnostic(d *tfprotov6.Diagnostic) {
	if p.diagSink == nil {
		return
	}
	msg := fmt.Sprintf("[%s] %s", d.Severity.String(), d.Detail)
	if d.Summary != "" {
		msg += fmt.Sprintf(": %s", d.Summary)
	}
	if d.Attribute != nil {
		msg += fmt.Sprintf(" at attribute %s", d.Attribute.String())
	}
	switch d.Severity {
	case tfprotov6.DiagnosticSeverityError, tfprotov6.DiagnosticSeverityInvalid:
		p.diagSink.Errorf(&diag.Diag{Message: msg})
	case tfprotov6.DiagnosticSeverityWarning:
		p.diagSink.Warningf(&diag.Diag{Message: msg})
	}
}
