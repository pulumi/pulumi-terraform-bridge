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
package crosstests

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/stretchr/testify/require"
)

func execCmd(t T, wdir string, environ []string, program string, args ...string) *exec.Cmd {
	t.Logf("%s %s", program, strings.Join(args, " "))
	cmd := exec.Command(program, args...)
	var stdout, stderr bytes.Buffer
	cmd.Dir = wdir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, environ...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(t, err, "error from `%s %s`\n\nStdout:\n%s\n\nStderr:\n%s\n\n",
		program, strings.Join(args, " "), stdout.String(), stderr.String())
	return cmd
}
