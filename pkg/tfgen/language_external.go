// Copyright 2016-2022, Pulumi Corporation.
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

// External languages allow extending tfgen with new SDK generators
// without liking them in as a build dependency of tfgen itself.
//
// If you specify "external:mylang" as the desired language, tfgen
// will search for `pulumi-tfgen-external-language-mylang` executable
// in PATH, and delegate language backend operations to that
// executable.
//
// To implement an external language, use `tfgen.ExternalLanguageMain`
// in a Go CLI program.

package tfgen

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type externalLanguageBackend struct {
	languageName string
}

var _ languageBackend = &externalLanguageBackend{}

func (b *externalLanguageBackend) validateName() error {
	var isAlpha = regexp.MustCompile(`^[-a-zA-Z][-a-zA-Z0-9]*$`).MatchString
	if !isAlpha(b.languageName) {
		return b.errorf("Invalid language name `%s`: must be alphanumeric and non-empty", b.languageName)
	}
	return nil
}

func (b *externalLanguageBackend) executableName() (string, error) {
	err := b.validateName()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pulumi-tfgen-external-language-%s", b.languageName), nil
}

func (b *externalLanguageBackend) exec(operation string, input json.RawMessage) (json.RawMessage, error) {
	exe, err := b.executableName()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, "-operation", operation)
	cmd.Stdin = bytes.NewBuffer(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, b.errorf("Failed to execute `%s -operation %s`: %w\n%s",
			exe, operation, err, out)
	}
	return out, err
}

func (b *externalLanguageBackend) name() string {
	return b.languageName
}

func (b *externalLanguageBackend) shouldConvertExamples() bool {
	return false
}

func (b *externalLanguageBackend) overlayInfo(info *tfbridge.ProviderInfo) *tfbridge.OverlayInfo {
	return nil
}

func (b *externalLanguageBackend) emitFiles(
	pulumiPackageSpec *pschema.PackageSpec,
	overlay *tfbridge.OverlayInfo,
	_ afero.Fs,
) (map[string][]byte, error) {

	if overlay != nil {
		return nil, b.errorf("Expecting overlay to be nil")
	}

	input, err := json.Marshal(pulumiPackageSpec)
	if err != nil {
		return nil, b.errorf("Cannot marshal PackageSpec to JSON: %v", err)
	}

	output, err := b.exec("emitFiles", input)
	if err != nil {
		return nil, err
	}

	var files map[string][]byte

	if err = json.Unmarshal(output, &files); err != nil {
		return nil, b.errorf("Cannot parse emitted files from JSON: %v", err)
	}

	return files, nil
}

func (b *externalLanguageBackend) errorf(format string, args ...interface{}) error {
	f := fmt.Sprintf("[externalLanguageBackend %s] %s", b.languageName, format)
	return fmt.Errorf(f, args...)
}

func parseExternalLanguage(lang Language) (languageBackend, error) {
	prefix := "external:"
	if strings.HasPrefix(string(lang), prefix) {
		lang := strings.TrimPrefix(string(lang), prefix)
		backend := &externalLanguageBackend{lang}
		if err := backend.validateName(); err != nil {
			return nil, err
		}
		return backend, nil
	}
	return nil, nil
}

func ExternalLanguageMain(emitFiles func(*pschema.PackageSpec) (map[string][]byte, error)) {
	operation := flag.String("operation", "", "operation to perform, typically emitFiles")
	flag.Parse()
	switch *operation {
	case "emitFiles":
		var pulumiPackageSpec *pschema.PackageSpec
		err := json.NewDecoder(os.Stdin).Decode(&pulumiPackageSpec)
		if err != nil {
			log.Fatal(fmt.Errorf("Cannot parse PackageSpec from JSON: %w", err))
		}
		files, err := emitFiles(pulumiPackageSpec)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(files); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal(fmt.Errorf("Required -operation, supported values: [emitFiles]"))
	}
}
