//go:build ignore

package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func main() {
	schema, err := exampleSchema()
	if err != nil {
		log.Fatal(err)
	}
	bytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	p := filepath.Join("testdata", "test-schema.json")
	err = os.WriteFile(p, bytes, 0600)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Wrote", p)
}

func exampleSchema() (pschema.PackageSpec, error) {
	return tfgen.GenerateSchema(info.Provider{
		P: shim.NewProvider(
			&schema.Provider{
				ResourcesMap: map[string]*schema.Resource{
					"web_acl": {
						Schema: map[string]*schema.Schema{
							"statement": webACLRootStatementSchema(3),
						},
					},
				},
			},
		),
		Name: "myprov",
		Resources: map[string]*info.Resource{
			"web_acl": {
				Tok: "myprov:index:WebAcl",
			},
		},
	}, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}))
}

func webACLRootStatementSchema(level int) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"and_statement":         statementSchema(level),
				"rate_based_statement":  rateBasedStatementSchema(level),
				"sqli_match_statement":  sqliMatchStatementSchema(),
				"regex_match_statement": regexMatchStatementSchema(),
			},
		},
	}
}

func rateBasedStatementSchema(level int) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"aggregate_key_type": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"scope_down_statement": scopeDownStatementSchema(level - 1),
			},
		},
	}
}

func regexMatchStatementSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"regex_string": {
					Type:     schema.TypeString,
					Required: true,
				},
				"field_to_match":      fieldToMatchSchema(),
				"text_transformation": textTransformationSchema(),
			},
		},
	}
}

func scopeDownStatementSchema(level int) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"and_statement":         statementSchema(level),
				"xss_match_statement":   xssMatchStatementSchema(),
				"sqli_match_statement":  sqliMatchStatementSchema(),
				"regex_match_statement": regexMatchStatementSchema(),
			},
		},
	}
}

func statementSchema(level int) *schema.Schema {
	if level > 1 {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"statement": {
						Type:     schema.TypeList,
						Required: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"and_statement":         statementSchema(level - 1),
								"xss_match_statement":   xssMatchStatementSchema(),
								"sqli_match_statement":  sqliMatchStatementSchema(),
								"regex_match_statement": regexMatchStatementSchema(),
							},
						},
					},
				},
			},
		}
	}

	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"statement": {
					Type:     schema.TypeList,
					Required: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"xss_match_statement":   xssMatchStatementSchema(),
							"sqli_match_statement":  sqliMatchStatementSchema(),
							"regex_match_statement": regexMatchStatementSchema(),
						},
					},
				},
			},
		},
	}
}

func xssMatchStatementSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"field_to_match": fieldToMatchSchema(),
			},
		},
	}
}

func fieldToMatchSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem:     fieldToMatchBaseSchema(),
	}
}

func sqliMatchStatementSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"field_to_match":      fieldToMatchSchema(),
				"text_transformation": textTransformationSchema(),
			},
		},
	}
}

const (
	namesAttrPriority = "priority"
	namesAttrType     = "type"
)

func textTransformationSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Required: true,
		MinItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				namesAttrPriority: {
					Type:     schema.TypeInt,
					Required: true,
				},
				namesAttrType: {
					Type:     schema.TypeString,
					Required: true,
				},
			},
		},
	}
}

var listOfEmptyObjectSchema *schema.Schema = &schema.Schema{
	Type:     schema.TypeList,
	Optional: true,
	MaxItems: 1,
	Elem: &schema.Resource{
		Schema: map[string]*schema.Schema{},
	},
}

func emptySchema() *schema.Schema {
	return listOfEmptyObjectSchema
}

func fieldToMatchBaseSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"method": emptySchema(),
		},
	}
}
