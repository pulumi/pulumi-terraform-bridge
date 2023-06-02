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

package tfbridgetests

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	exitCode, err := testMain(m)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(exitCode)
}

func testMain(m *testing.M) (exitCode int, err error) {
	err = setupUnitTests()
	if err != nil {
		return exitCode, err
	}

	defer func() {
		panicResult := recover()
		var panicError error
		if panicResult != nil {
			panicError = fmt.Errorf("Ignoring panic in tests: %v", panicResult)
		}
		teardownError := teardownUnitTests()
		err = multierror.Append(panicError, teardownError)
	}()

	exitCode = m.Run()
	return exitCode, err
}

func setupUnitTests() error {
	return os.MkdirAll("state", os.ModePerm)
}

func teardownUnitTests() error {
	return os.RemoveAll("state")
}
