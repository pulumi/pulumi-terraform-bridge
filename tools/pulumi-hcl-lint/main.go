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
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
)

func main() {
	ctx := context.Background()
	useJSONRef := flag.Bool("json", false, "Emit output in JSON format")
	outRef := flag.String("out", "", "Emit output to the given file")
	flag.Parse()
	useJSON := *useJSONRef
	out := *outRef

	var outWriter io.Writer
	if out == "" {
		outWriter = os.Stdout
	} else {
		f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open output file: %v", err)
			os.Exit(1)
		}
		defer f.Close()
		outWriter = f
	}

	var logger *slog.Logger
	if useJSON {
		logger = slog.New(slog.NewJSONHandler(outWriter, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(outWriter, nil))
	}

	issues := make(chan issue)

	go func() {
		if err := lint(ctx, ".", issues); err != nil {
			logger.Error("Unexpected failure: %s", err)
			os.Exit(1)
		}
		close(issues)
	}()

	for i := range issues {
		attrs := []slog.Attr{{
			Key:   "code",
			Value: slog.StringValue(i.Code()),
		}}

		for k, v := range i.Attributes() {
			attrs = append(attrs, slog.Attr{
				Key:   k,
				Value: slog.StringValue(v),
			})
		}
		logger.LogAttrs(ctx, slog.LevelError, i.Detail(), attrs...)
	}
}
