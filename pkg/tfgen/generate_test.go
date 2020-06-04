package tfgen

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/stretchr/testify/assert"
)

func Test_DeprecationFromTFSchema(t *testing.T) {
	v := &variable{
		name:   "v",
		schema: &schema.Schema{Type: schema.TypeString, Deprecated: "This is deprecated"},
	}

	deprecationMessage := v.deprecationMessage()
	assert.Equal(t, "This is deprecated", deprecationMessage)
}
