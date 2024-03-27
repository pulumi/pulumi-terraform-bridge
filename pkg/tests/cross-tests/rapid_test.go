package crosstests

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"pgregory.net/rapid"
)

func TestDiffConvergence(outerT *testing.T) {
	tvg := &tvGen{}

	rapid.Check(outerT, func(t *rapid.T) {
		tv := tvg.GenObject(3).Draw(t, "tv")

		c1 := tv.valueGen.Draw(t, "config1")
		c2 := tv.valueGen.Draw(t, "config2")

		tc := diffTestCase{
			Resource: &schema.Resource{
				Schema: tv.schema.Elem.(*schema.Resource).Schema,
			},
			Config1: c1,
			Config2: c2,
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

func (*rapidTWithCleanup) Cleanup(work func()) {
	panic("unexpected cleanup scheduled")
}
