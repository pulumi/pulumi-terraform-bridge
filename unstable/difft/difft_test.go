package difft

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestDiffAppliesCorrectly(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s1 := rapid.StringMatching(`^[abc]{0,5}`).Draw(t, "s1")
		s2 := rapid.StringMatching(`^[abc]{0,5}`).Draw(t, "s2")

		eq := func(b1, b2 byte) bool {
			return b1 == b2
		}
		edits := DiffT([]byte(s1), []byte(s2), DiffTOptions[byte]{
			Equals: eq,
		})

		t.Logf("edits:")
		for _, ed := range edits {
			t.Logf("  %v", ed)
		}

		s3, err := edits.apply([]byte(s1), eq)
		if err != nil {
			t.Fatalf("apply failed: %v", err)
		}
		t.Logf("s3: %v", s3)
		if string(s3) != s2 {
			t.Fatalf("reconstructed string does not match: %q != %q", s3, s3)
		}
	})
}

func TestDiff(t *testing.T) {
	eq := func(a, b byte) bool {
		return a == b
	}
	input := []byte(`mario`)
	dd := DiffT(input, []byte(`darius`), DiffTOptions[byte]{Equals: eq})
	assert.Equal(t, Remove, dd[0].Change)
	assert.Equal(t, Insert, dd[1].Change)
	assert.Equal(t, Keep, dd[2].Change)
	assert.Equal(t, Keep, dd[3].Change)
	assert.Equal(t, Keep, dd[4].Change)
	assert.Equal(t, Remove, dd[5].Change)
	assert.Equal(t, Insert, dd[6].Change)
	assert.Equal(t, Insert, dd[7].Change)
}
