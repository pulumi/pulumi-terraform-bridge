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
		tv := tvg.GenBlock(3).Draw(t, "tv")

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
