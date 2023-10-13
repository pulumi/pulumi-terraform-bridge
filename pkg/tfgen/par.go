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

package tfgen

import (
	"fmt"
	"runtime"
	"sync"
)

// Transforms a large map in batches of up to batch elements, using workers number of goroutines. If
// workers is -1, uses one worker per CPU.
func parTransformMap[K comparable, T any, U any](
	inputs map[K]T,
	transform func(map[K]T) (map[K]U, error),
	workers int,
	batch int,
) (map[K]U, error) {
	if batch < 1 {
		return nil, fmt.Errorf("batch cannot be less than 1")
	}
	n := workers
	if workers < 1 {
		n = runtime.NumCPU()
		if n < 2 {
			n = 2
		}
	}

	keys := []K{}
	keyIndex := map[K]int{}
	for k := range inputs {
		keys = append(keys, k)
		keyIndex[k] = len(keys) - 1
	}

	translations := make([]U, len(keys))
	errors := make([]error, n)

	ch := make(chan []K)

	// Start n workers to do convertViaPulumiCLI work
	wg := sync.WaitGroup{}
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(worker int) {
			defer wg.Done()
			for keyBatch := range ch {
				ex := map[K]T{}
				for _, k := range keyBatch {
					ex[k] = inputs[k]
				}
				out, err := transform(ex)
				if err != nil {
					errors[worker] = err
					return
				}
				for _, k := range keyBatch {
					translations[keyIndex[k]] = out[k]
				}
			}
		}(i)
	}

	// Queue up work in batches.
	remainingKeys := keys
	for len(remainingKeys) > 0 {
		var keyBatch []K
		if len(remainingKeys) <= batch {
			keyBatch, remainingKeys = remainingKeys, nil
		} else {
			keyBatch, remainingKeys = remainingKeys[:batch], remainingKeys[batch:]
		}
		ch <- keyBatch
	}
	close(ch)

	// Wait till workers are done.
	wg.Wait()

	for _, e := range errors {
		// Returning the first error here, could instead consider merging them.
		if e != nil {
			return nil, e
		}
	}

	translatedMap := map[K]U{}
	for _, k := range keys {
		translatedMap[k] = translations[keyIndex[k]]
	}
	return translatedMap, nil
}
