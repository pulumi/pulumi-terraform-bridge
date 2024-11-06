package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/zclconf/go-cty/cty"
)

func TestDetailedDiffListAttribute(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	blockSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	blockReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	blockNestedReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
			},
		},
	}

	attrList := func(el ...string) cty.Value {
		slice := make([]cty.Value, len(el))
		for i, v := range el {
			slice[i] = cty.StringVal(v)
		}
		if len(slice) == 0 {
			return cty.ListValEmpty(cty.String)
		}
		return cty.ListVal(slice)
	}

	blockList := func(el ...string) cty.Value {
		slice := make([]cty.Value, len(el))
		for i, v := range el {
			slice[i] = cty.ObjectVal(
				map[string]cty.Value{
					"nested": cty.StringVal(v),
				},
			)
		}
		if len(slice) == 0 {
			return cty.ListValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
		}
		return cty.ListVal(slice)
	}

	t.Run("unchanged non-empty", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      ~ key = [
          ~ "value" -> "value1",
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      ~ key = [ # forces replacement
          ~ "value" -> "value1",
        ]
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: ~keys]
 +- testprovider:index:Test p replace [diff: ~keys]
 -- testprovider:index:Test p delete original [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      ~ key {
          ~ nested = "value" -> "value1"
        }
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      ~ key { # forces replacement
          ~ nested = "value" -> "value1"
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: ~keys]
 +- testprovider:index:Test p replace [diff: ~keys]
 -- testprovider:index:Test p delete original [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      + key = [
          + "value",
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      + key = [ # forces replacement
          + "value",
        ]
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: +keys]
 +- testprovider:index:Test p replace [diff: +keys]
 -- testprovider:index:Test p delete original [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      + key {
          + nested = "value"
        }
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      + key { # forces replacement
          + nested = "value"
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: +keys]
 +- testprovider:index:Test p replace [diff: +keys]
 -- testprovider:index:Test p delete original [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      + key {
          + nested = "value" # forces replacement
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: +keys]
 +- testprovider:index:Test p replace [diff: +keys]
 -- testprovider:index:Test p delete original [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      - key = [
          - "value",
        ] -> null
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      - key = [ # forces replacement
          - "value",
        ] -> null
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: -keys]
 +- testprovider:index:Test p replace [diff: -keys]
 -- testprovider:index:Test p delete original [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      - key {
          - nested = "value" -> null
        }
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      - key { # forces replacement
          - nested = "value" -> null
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: -keys]
 +- testprovider:index:Test p replace [diff: -keys]
 -- testprovider:index:Test p delete original [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      - key {
          - nested = "value" -> null
        }
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("null unchanged", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
				map[string]cty.Value{},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
				map[string]cty.Value{},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("null to non-null", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      + key = [
          + "value",
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      + key = [ # forces replacement
          + "value",
        ]
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: +keys]
 +- testprovider:index:Test p replace [diff: +keys]
 -- testprovider:index:Test p delete original [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      + key {
          + nested = "value"
        }
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      + key { # forces replacement
          + nested = "value"
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: +keys]
 +- testprovider:index:Test p replace [diff: +keys]
 -- testprovider:index:Test p delete original [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      + key {
          + nested = "value" # forces replacement
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: +keys]
 +- testprovider:index:Test p replace [diff: +keys]
 -- testprovider:index:Test p delete original [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("non-null to null", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      - key = [
          - "value",
        ] -> null
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      - key = [ # forces replacement
          - "value",
        ] -> null
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: -keys]
 +- testprovider:index:Test p replace [diff: -keys]
 -- testprovider:index:Test p delete original [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      - key {
          - nested = "value" -> null
        }
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      - key { # forces replacement
          - nested = "value" -> null
        }
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: -keys]
 +- testprovider:index:Test p replace [diff: -keys]
 -- testprovider:index:Test p delete original [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      - key {
          - nested = "value" -> null
        }
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed null to empty", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList()},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      + key = []
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList()},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      + key = [] # forces replacement
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: +keys]
 +- testprovider:index:Test p replace [diff: +keys]
 -- testprovider:index:Test p delete original [diff: +keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
				map[string]cty.Value{"key": blockList()},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
				map[string]cty.Value{"key": blockList()},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed empty to null", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList()},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      - key = [] -> null
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList()},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      - key = [] -> null # forces replacement
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: -keys]
 +- testprovider:index:Test p replace [diff: -keys]
 -- testprovider:index:Test p delete original [diff: -keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList()},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList()},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList()},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.Object(map[string]cty.Type{"nested": cty.String})))},
			)

			autogold.Expect(`
No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.
`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

    pulumi:pulumi:Stack project-test
Resources:
    2 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("element added", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value", "value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      ~ key = [
            "value",
          + "value1",
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value", "value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      ~ key = [ # forces replacement
            "value",
          + "value1",
        ]
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: ~keys]
 +- testprovider:index:Test p replace [diff: ~keys]
 -- testprovider:index:Test p delete original [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value", "value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      + key {
          + nested = "value1"
        }

        # (1 unchanged block hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value", "value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      + key { # forces replacement
          + nested = "value1"
        }

        # (1 unchanged block hidden)
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: ~keys]
 +- testprovider:index:Test p replace [diff: ~keys]
 -- testprovider:index:Test p delete original [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value", "value1")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      + key {
          + nested = "value1" # forces replacement
        }

        # (1 unchanged block hidden)
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: ~keys]
 +- testprovider:index:Test p replace [diff: ~keys]
 -- testprovider:index:Test p delete original [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("element removed", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value", "value1")},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      ~ key = [
            "value",
          - "value1",
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value", "value1")},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id  = "test-id" -> (known after apply)
      ~ key = [ # forces replacement
            "value",
          - "value1",
        ]
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: ~keys]
 +- testprovider:index:Test p replace [diff: ~keys]
 -- testprovider:index:Test p delete original [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value", "value1")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      - key {
          - nested = "value1" -> null
        }

        # (1 unchanged block hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value", "value1")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
+/- create replacement and then destroy

Terraform will perform the following actions:

  # testprovider_test.res must be replaced
+/- resource "testprovider_test" "res" {
      ~ id = "test-id" -> (known after apply)

      - key { # forces replacement
          - nested = "value1" -> null
        }

        # (1 unchanged block hidden)
    }

Plan: 1 to add, 0 to change, 1 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ++ testprovider:index:Test p create replacement [diff: ~keys]
 +- testprovider:index:Test p replace [diff: ~keys]
 -- testprovider:index:Test p delete original [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    +-1 to replace
    1 unchanged

`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value", "value1")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id = "test-id"

      - key {
          - nested = "value1" -> null
        }

        # (1 unchanged block hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, res.TFOut)
			autogold.Expect(`Previewing update (test):

 ~  testprovider:index:Test p update [diff: ~keys]
    pulumi:pulumi:Stack project-test
Resources:
    ~ 1 to update
    1 unchanged

`).Equal(t, res.PulumiOut)
		})
	})
}
