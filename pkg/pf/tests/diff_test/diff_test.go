package tfbridgetests

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
)

func TestPFSimpleNoDiff(t *testing.T) {
	t.Parallel()

	sch := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{Optional: true},
		},
	}

	res := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch,
	})
	diff := crosstests.Diff(t, res,
		map[string]cty.Value{"key": cty.StringVal("value")},
		map[string]cty.Value{"key": cty.StringVal("value1")},
	)

	autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      ~ key = "value" -> "value1"
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, diff.TFOut)
	autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::project::pulumi:pulumi:Stack::project-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::project::testprovider:index/test:Test::p]
      ~ key: "value" => "value1"
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, diff.PulumiOut)
}

func TestPFGitlabDiffRepro(t *testing.T) {
	t.Parallel()

	getSchema := func(withNew bool) rschema.Schema {
		attributes := map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				MarkdownDescription: `The id of the project hook. In the format of "project:hook_id"`,
				Computed:            true,
			},
			"project": rschema.StringAttribute{
				MarkdownDescription: "The name or id of the project to add the hook to.",
				Required:            true,
			},
			"project_id": rschema.Int64Attribute{
				MarkdownDescription: "The id of the project for the hook.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"hook_id": rschema.Int64Attribute{
				MarkdownDescription: "The id of the project hook.",
				Computed:            true,
			},
			"url": rschema.StringAttribute{
				MarkdownDescription: "The url of the hook to invoke. Forces re-creation to preserve `token`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^\S+$`), `The URL may not contain whitespace`),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token": rschema.StringAttribute{
				MarkdownDescription: "A token to present when invoking the hook. The token is not available for imported resources.",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
			},
			"name": rschema.StringAttribute{
				MarkdownDescription: "Name of the project webhook.",
				Optional:            true,
				Computed:            true,
			},
			"description": rschema.StringAttribute{
				MarkdownDescription: "Description of the webhook.",
				Optional:            true,
				Computed:            true,
			},
			"push_events": rschema.BoolAttribute{
				Description: "Invoke the hook for push events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"push_events_branch_filter": rschema.StringAttribute{
				Description: "Invoke the hook for push events on matching branches only.",
				Optional:    true,
				Computed:    true,
			},
			"issues_events": rschema.BoolAttribute{
				Description: "Invoke the hook for issues events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"confidential_issues_events": rschema.BoolAttribute{
				Description: "Invoke the hook for confidential issues events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"merge_requests_events": rschema.BoolAttribute{
				Description: "Invoke the hook for merge requests events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"tag_push_events": rschema.BoolAttribute{
				Description: "Invoke the hook for tag push events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"note_events": rschema.BoolAttribute{
				Description: "Invoke the hook for note events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"confidential_note_events": rschema.BoolAttribute{
				Description: "Invoke the hook for confidential note events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"job_events": rschema.BoolAttribute{
				Description: "Invoke the hook for job events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"pipeline_events": rschema.BoolAttribute{
				Description: "Invoke the hook for pipeline events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"wiki_page_events": rschema.BoolAttribute{
				Description: "Invoke the hook for wiki page events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"deployment_events": rschema.BoolAttribute{
				Description: "Invoke the hook for deployment events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"releases_events": rschema.BoolAttribute{
				Description: "Invoke the hook for release events.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"enable_ssl_verification": rschema.BoolAttribute{
				Description: "Enable SSL verification when invoking the hook.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"custom_webhook_template": rschema.StringAttribute{
				Description: "Custom webhook template.",
				Optional:    true,
				Computed:    true,
			},
			"custom_headers": rschema.ListNestedAttribute{
				Description: "Custom headers for the project webhook.",
				Optional:    true,
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"key": rschema.StringAttribute{
							Description: "Key of the custom header.",
							Required:    true,
						},
						"value": rschema.StringAttribute{
							Required:      true,
							Description:   "Value of the custom header. This value cannot be imported.",
							Sensitive:     true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						},
					},
				},
			},
		}
		if withNew {
			attributes["resource_access_token_events"] = rschema.BoolAttribute{
				Default:  booldefault.StaticBool(false),
				Computed: true,
				Optional: true,
			}
		}
		return rschema.Schema{
			Attributes: attributes,
		}
	}

	res := pb.NewResource(pb.NewResourceArgs{
		CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
			resp.State = tfsdk.State(req.Config)
			resp.State.SetAttribute(ctx, path.Root("project_id"), 123)
			resp.State.SetAttribute(ctx, path.Root("hook_id"), 567)
			resp.State.SetAttribute(ctx, path.Root("id"), "abc")
			resp.State.SetAttribute(ctx, path.Root("enable_ssl_verification"), true)
			resp.State.SetAttribute(ctx, path.Root("confidential_note_events"), false)
			resp.State.SetAttribute(ctx, path.Root("releases_events"), false)
			resp.State.SetAttribute(ctx, path.Root("deployment_events"), false)
		},
		UpdateFunc: func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
			resp.State = tfsdk.State(req.Plan)
		},
		ResourceSchema: getSchema(false),
	})

	inputs := map[string]cty.Value{
		"project":                    cty.StringVal("project_id"),
		"name":                       cty.StringVal("webhook-receiver"),
		"confidential_issues_events": cty.BoolVal(true),
		"issues_events":              cty.BoolVal(true),
		"job_events":                 cty.BoolVal(true),
		"merge_requests_events":      cty.BoolVal(true),
		"note_events":                cty.BoolVal(true),
		"pipeline_events":            cty.BoolVal(true),
		"push_events":                cty.BoolVal(true),
		"tag_push_events":            cty.BoolVal(true),
		"url":                        cty.StringVal("https://webhook.receiver.endpoint/hooks/gitlab"),
		"wiki_page_events":           cty.BoolVal(true),
	}
	diff := crosstests.Diff(t, res,
		inputs,
		inputs,
		crosstests.DiffUpdateResource(pb.NewResource(pb.NewResourceArgs{
			ResourceSchema: getSchema(true),
		})),
		crosstests.DiffSkipUp(true),
	)

	autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      + custom_webhook_template      = (known after apply)
      + description                  = (known after apply)
      ~ hook_id                      = 567 -> (known after apply)
      ~ id                           = "abc" -> (known after apply)
        name                         = "webhook-receiver"
      ~ project_id                   = 123 -> (known after apply) # forces replacement
      + push_events_branch_filter    = (known after apply)
      + resource_access_token_events = false
      + token                        = (sensitive value)
        # (15 unchanged attributes hidden)
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, diff.TFOut)
	autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::project::pulumi:pulumi:Stack::project-test]
    +-testprovider:index/test:Test: (replace)
        [id=abc]
        [urn=urn:pulumi:test::project::testprovider:index/test:Test::p]
        confidentialIssuesEvents : true
        confidentialNoteEvents   : false
      + customWebhookTemplate    : output<string>
        deploymentEvents         : false
      + description              : output<string>
        enableSslVerification    : true
      ~ hookId                   : 567 => output<string>
      ~ id                       : "abc" => output<string>
        issuesEvents             : true
        jobEvents                : true
        mergeRequestsEvents      : true
        name                     : "webhook-receiver"
        noteEvents               : true
        pipelineEvents           : true
        project                  : "project_id"
      ~ projectId                : 123 => output<string>
        pushEvents               : true
      + pushEventsBranchFilter   : output<string>
        releasesEvents           : false
      + resourceAccessTokenEvents: false
        tagPushEvents            : true
      + token                    : [secret]
        url                      : "https://webhook.receiver.endpoint/hooks/gitlab"
        wikiPageEvents           : true
Resources:
    +-1 to replace
    1 unchanged
`).Equal(t, diff.PulumiOut)
}

func TestPFDetailedDiffStringAttribute(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{Optional: true},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}

	attributeSchemaWithDefault := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringDefault("default"),
			},
		},
	}

	attributeSchemaWithDefaultReplace := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringDefault("default"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}

	attributeSchemaWitPlanModifierDefault := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringDefault("default")},
			},
		},
	}

	attributeSchemaWithPlanModifierDefaultReplace := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringDefault("default"), stringplanmodifier.RequiresReplace()},
			},
		},
	}

	schemas := []struct {
		name   string
		schema rschema.Schema
	}{
		{"no replace", attributeSchema},
		{"replace", attributeReplaceSchema},
		{"default", attributeSchemaWithDefault},
		{"default replace", attributeSchemaWithDefaultReplace},
		{"plan modifier default", attributeSchemaWitPlanModifierDefault},
		{"plan modifier default replace", attributeSchemaWithPlanModifierDefaultReplace},
	}

	makeValue := func(s *string) cty.Value {
		if s == nil {
			return cty.NullVal(cty.DynamicPseudoType)
		}
		return cty.StringVal(*s)
	}

	scenarios := []struct {
		name         string
		initialValue *string
		changeValue  *string
	}{
		{"unchanged", ref("value"), ref("value")},
		{"changed", ref("value"), ref("value1")},
		{"added", nil, ref("value")},
		{"removed", ref("value"), nil},
	}

	for _, schema := range schemas {
		t.Run(schema.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := makeValue(scenario.initialValue)
					changeValue := makeValue(scenario.changeValue)

					res := pb.NewResource(pb.NewResourceArgs{
						ResourceSchema: schema.schema,
					})
					diff := crosstests.Diff(
						t, res, map[string]cty.Value{"key": initialValue}, map[string]cty.Value{"key": changeValue},
					)

					autogold.ExpectFile(t, testOutput{
						initialValue: scenario.initialValue,
						changeValue:  scenario.changeValue,
						tfOut:        diff.TFOut,
						pulumiOut:    diff.PulumiOut,
						detailedDiff: diff.PulumiDiff.DetailedDiff,
					})
				})
			}
		})
	}
}
