// Copyright 2016-2024, Pulumi Corporation.
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

// Helpers to execute OS commands.
package tfcheck

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/stretchr/testify/require"
)

func (d *TFDriver) execTf(t pulcheck.T, args ...string) ([]byte, error) {
	cmd, err := execCmd(t, d.cwd, []string{d.formatReattachEnvVar()}, getTFCommand(), args...)
	if stderr := cmd.Stderr.(*bytes.Buffer).String(); len(stderr) > 0 {
		t.Logf("%q stderr:\n%s\n", cmd.String(), stderr)
	}
	return cmd.Stdout.(*bytes.Buffer).Bytes(), err
}

func execCmd(t pulcheck.T, wdir string, environ []string, program string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(program, args...)
	require.NoError(t, cmd.Err)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = wdir
	cmd.Env = append(os.Environ(), environ...)
	t.Logf("%s", cmd.String())
	err := cmd.Run()
	if err != nil {
		t.Logf("error from %q\n\nStdout:\n%s\n\nStderr:\n%s\n\n",
			cmd.String(), stdout.String(), stderr.String())
	}
	return cmd, err
}
