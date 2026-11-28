// Copyright 2026, Pulumi Corporation.
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

package info

// CommandRemediation is a command that the user can run to resolve a configuration check failure, such as
// signing in to a cloud. The engine shows the command to the user and runs it only with their consent.
type CommandRemediation struct {
	// Description is a short, imperative summary of what the command does, such as "Sign in to AWS".
	Description string
	// Command is the program and its arguments. The program is looked up on PATH and run without a shell,
	// attached to the user's terminal.
	Command []string
	// Env holds environment variables to set for the command, in addition to the engine's environment.
	Env map[string]string
}
