package config

import (
	"testing"
)

func TestHCLConfigurableConfigurable(t *testing.T) {
    t.Parallel()
	var _ configurable = new(hclConfigurable)
}
