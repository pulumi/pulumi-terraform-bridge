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

// Rapid-driven property-based tests. These allow randomized exploration of the schema space and locating
// counter-examples.
package crosstests

import (
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"pgregory.net/rapid"
)

func TestDiffConvergence(outerT *testing.T) {
	_, ok := os.LookupEnv("PULUMI_EXPERIMENTAL")
	if !ok {
		outerT.Skip("TODO - we do not currently pass all cases; using this as an exploration tool")
	}
	outerT.Parallel()

	log.SetOutput(io.Discard)
	tvg := &tvGen{}

	rapid.Check(outerT, func(t *rapid.T) {
		outerT.Logf("Iterating..")
		tv := tvg.GenBlockWithDepth(3, "").Draw(t, "tv")

		t.Logf("Schema:\n%v\n", (&prettySchemaWrapper{schema.Schema{Elem: &schema.Resource{
			Schema: tv.schemaMap,
		}}}).GoString())

		c1 := rapid.Map(tv.valueGen, newPrettyValueWrapper).Draw(t, "config1").Value()
		c2 := rapid.Map(tv.valueGen, newPrettyValueWrapper).Draw(t, "config2").Value()
		ty := tv.typ

		tc := diffTestCase{
			Resource: &schema.Resource{
				Schema: tv.schemaMap,
			},
			Config1:    c1,
			Config2:    c2,
			ObjectType: &ty,
		}

		runDiffCheck(&rapidTWithCleanup{t, outerT}, tc)
	})
}

func TestCreateInputsConvergence(outerT *testing.T) {
	_, ok := os.LookupEnv("PULUMI_EXPERIMENTAL")
	if !ok {
		outerT.Skip("TODO - we do not currently pass all cases; using this as an exploration tool")
	}
	outerT.Parallel()

	log.SetOutput(io.Discard)
	typedValGenerator := &tvGen{
		skipNullCollections: true,
	}

	rapid.Check(outerT, func(t *rapid.T) {
		outerT.Logf("Iterating..")
		depth := rapid.IntRange(1, 3).Draw(t, "schemaDepth")
		tv := typedValGenerator.GenBlockWithDepth(depth, "").Draw(t, "tv")

		t.Logf("Schema:\n%v\n", (&prettySchemaWrapper{schema.Schema{Elem: &schema.Resource{
			Schema: tv.schemaMap,
		}}}).GoString())

		config := rapid.Map(tv.valueGen, newPrettyValueWrapper).Draw(t, "config1").Value()
		ty := tv.typ

		tc := inputTestCase{
			Resource: &schema.Resource{
				Schema: tv.schemaMap,
			},
			Config:     config,
			ObjectType: &ty,
		}

		runCreateInputCheck(&rapidTWithCleanup{t, outerT}, tc)
	})
}

type rapidTWithCleanup struct {
	*rapid.T
	outerT *testing.T
}

func (rtc *rapidTWithCleanup) TempDir() string {
	return rtc.outerT.TempDir()
}

func (*rapidTWithCleanup) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (rtc *rapidTWithCleanup) Cleanup(work func()) {
	rtc.outerT.Cleanup(work)
}
