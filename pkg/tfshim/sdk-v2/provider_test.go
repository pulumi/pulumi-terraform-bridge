package sdkv2

import (
	"context"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider1UpgradeResourceState(t *testing.T) {
	t.Parallel()

	type tc struct {
		name   string
		schema *schema.Resource
		input  func() *terraform.InstanceState
		expect func(t *testing.T, actual *terraform.InstanceState, tc tc)
	}

	tests := []tc{
		{
			name: "roundtrip int64",
			schema: &schema.Resource{
				UseJSONNumber: true,
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			input: func() *terraform.InstanceState {
				n, err := cty.ParseNumberVal("641577219598130723")
				require.NoError(t, err)
				v := cty.ObjectVal(map[string]cty.Value{"x": n})
				s := terraform.NewInstanceStateShimmedFromValue(v, 0)
				s.Meta["schema_version"] = "0"
				s.ID = "id"
				s.RawState = v
				s.Attributes["id"] = s.ID
				return s
			},
			expect: func(t *testing.T, actual *terraform.InstanceState, tc tc) {
				assert.Equal(t, tc.input().Attributes, actual.Attributes)
			},
		},
		{
			name: "type change",
			schema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x1": {Type: schema.TypeInt, Optional: true},
				},
				SchemaVersion: 1,
				StateUpgraders: []schema.StateUpgrader{{
					Version: 0,
					Type: cty.Object(map[string]cty.Type{
						"id": cty.String,
						"x0": cty.String,
					}),
					Upgrade: func(_ context.Context, rawState map[string]any, _ interface{}) (map[string]any, error) {
						return map[string]any{
							"id": rawState["id"],
							"x1": len(rawState["x0"].(string)),
						}, nil
					},
				}},
			},
			input: func() *terraform.InstanceState {
				s := terraform.NewInstanceStateShimmedFromValue(cty.ObjectVal(map[string]cty.Value{
					"x0": cty.StringVal("123"),
				}), 0)
				s.Meta["schema_version"] = "0"
				s.ID = "id"
				return s
			},
			expect: func(t *testing.T, actual *terraform.InstanceState, tc tc) {
				t.Logf("Actual = %#v", actual)
				assert.Equal(t, map[string]string{
					"id": "id",
					"x1": "3",
				}, actual.Attributes)
			},
		},
	}

	const tfToken = "test_token"

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			require.NoError(t, tt.schema.InternalValidate(tt.schema.Schema, true))

			p := &schema.Provider{ResourcesMap: map[string]*schema.Resource{tfToken: tt.schema}}

			actual, err := upgradeResourceState(ctx, tfToken, p, tt.schema, tt.input())
			require.NoError(t, err)

			tt.expect(t, actual, tt)
		})
	}
}

func TestProviderDetailedSchemaDump(t *testing.T) {
	prov := NewProvider(&schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"foo": {Type: schema.TypeString},
					"bar": {Type: schema.TypeInt},
				},
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"test_data_source": {
				Schema: map[string]*schema.Schema{
					"foo": {Type: schema.TypeString},
					"bar": {Type: schema.TypeInt},
				},
			},
		},
		Schema: map[string]*schema.Schema{
			"test_schema": {Type: schema.TypeString},
		},
	})

	autogold.Expect(`(*schema.Provider)({
 Schema: (map[string]*schema.Schema) (len=1) {
  (string) (len=11) "test_schema": (*schema.Schema)({
   Type: (schema.ValueType) TypeString,
   ConfigMode: (schema.SchemaConfigMode) 0,
   Required: (bool) false,
   Optional: (bool) false,
   Computed: (bool) false,
   ForceNew: (bool) false,
   DiffSuppressFunc: (schema.SchemaDiffSuppressFunc) <nil>,
   DiffSuppressOnRefresh: (bool) false,
   Default: (interface {}) <nil>,
   DefaultFunc: (schema.SchemaDefaultFunc) <nil>,
   Description: (string) "",
   InputDefault: (string) "",
   StateFunc: (schema.SchemaStateFunc) <nil>,
   Elem: (interface {}) <nil>,
   MaxItems: (int) 0,
   MinItems: (int) 0,
   Set: (schema.SchemaSetFunc) <nil>,
   ComputedWhen: ([]string) <nil>,
   ConflictsWith: ([]string) <nil>,
   ExactlyOneOf: ([]string) <nil>,
   AtLeastOneOf: ([]string) <nil>,
   RequiredWith: ([]string) <nil>,
   Deprecated: (string) "",
   ValidateFunc: (schema.SchemaValidateFunc) <nil>,
   ValidateDiagFunc: (schema.SchemaValidateDiagFunc) <nil>,
   Sensitive: (bool) false
  })
 },
 ResourcesMap: (map[string]*schema.Resource) (len=1) {
  (string) (len=13) "test_resource": (*schema.Resource)({
   Schema: (map[string]*schema.Schema) (len=2) {
    (string) (len=3) "foo": (*schema.Schema)({
     Type: (schema.ValueType) TypeString,
     ConfigMode: (schema.SchemaConfigMode) 0,
     Required: (bool) false,
     Optional: (bool) false,
     Computed: (bool) false,
     ForceNew: (bool) false,
     DiffSuppressFunc: (schema.SchemaDiffSuppressFunc) <nil>,
     DiffSuppressOnRefresh: (bool) false,
     Default: (interface {}) <nil>,
     DefaultFunc: (schema.SchemaDefaultFunc) <nil>,
     Description: (string) "",
     InputDefault: (string) "",
     StateFunc: (schema.SchemaStateFunc) <nil>,
     Elem: (interface {}) <nil>,
     MaxItems: (int) 0,
     MinItems: (int) 0,
     Set: (schema.SchemaSetFunc) <nil>,
     ComputedWhen: ([]string) <nil>,
     ConflictsWith: ([]string) <nil>,
     ExactlyOneOf: ([]string) <nil>,
     AtLeastOneOf: ([]string) <nil>,
     RequiredWith: ([]string) <nil>,
     Deprecated: (string) "",
     ValidateFunc: (schema.SchemaValidateFunc) <nil>,
     ValidateDiagFunc: (schema.SchemaValidateDiagFunc) <nil>,
     Sensitive: (bool) false
    }),
    (string) (len=3) "bar": (*schema.Schema)({
     Type: (schema.ValueType) TypeInt,
     ConfigMode: (schema.SchemaConfigMode) 0,
     Required: (bool) false,
     Optional: (bool) false,
     Computed: (bool) false,
     ForceNew: (bool) false,
     DiffSuppressFunc: (schema.SchemaDiffSuppressFunc) <nil>,
     DiffSuppressOnRefresh: (bool) false,
     Default: (interface {}) <nil>,
     DefaultFunc: (schema.SchemaDefaultFunc) <nil>,
     Description: (string) "",
     InputDefault: (string) "",
     StateFunc: (schema.SchemaStateFunc) <nil>,
     Elem: (interface {}) <nil>,
     MaxItems: (int) 0,
     MinItems: (int) 0,
     Set: (schema.SchemaSetFunc) <nil>,
     ComputedWhen: ([]string) <nil>,
     ConflictsWith: ([]string) <nil>,
     ExactlyOneOf: ([]string) <nil>,
     AtLeastOneOf: ([]string) <nil>,
     RequiredWith: ([]string) <nil>,
     Deprecated: (string) "",
     ValidateFunc: (schema.SchemaValidateFunc) <nil>,
     ValidateDiagFunc: (schema.SchemaValidateDiagFunc) <nil>,
     Sensitive: (bool) false
    })
   },
   SchemaFunc: (func() map[string]*schema.Schema) <nil>,
   SchemaVersion: (int) 0,
   MigrateState: (schema.StateMigrateFunc) <nil>,
   StateUpgraders: ([]schema.StateUpgrader) <nil>,
   Create: (schema.CreateFunc) <nil>,
   Read: (schema.ReadFunc) <nil>,
   Update: (schema.UpdateFunc) <nil>,
   Delete: (schema.DeleteFunc) <nil>,
   Exists: (schema.ExistsFunc) <nil>,
   CreateContext: (schema.CreateContextFunc) <nil>,
   ReadContext: (schema.ReadContextFunc) <nil>,
   UpdateContext: (schema.UpdateContextFunc) <nil>,
   DeleteContext: (schema.DeleteContextFunc) <nil>,
   CreateWithoutTimeout: (schema.CreateContextFunc) <nil>,
   ReadWithoutTimeout: (schema.ReadContextFunc) <nil>,
   UpdateWithoutTimeout: (schema.UpdateContextFunc) <nil>,
   DeleteWithoutTimeout: (schema.DeleteContextFunc) <nil>,
   CustomizeDiff: (schema.CustomizeDiffFunc) <nil>,
   Importer: (*schema.ResourceImporter)(<nil>),
   DeprecationMessage: (string) "",
   Timeouts: (*schema.ResourceTimeout)(<nil>),
   Description: (string) "",
   UseJSONNumber: (bool) false,
   EnableLegacyTypeSystemApplyErrors: (bool) false,
   EnableLegacyTypeSystemPlanErrors: (bool) false
  })
 },
 DataSourcesMap: (map[string]*schema.Resource) (len=1) {
  (string) (len=16) "test_data_source": (*schema.Resource)({
   Schema: (map[string]*schema.Schema) (len=2) {
    (string) (len=3) "foo": (*schema.Schema)({
     Type: (schema.ValueType) TypeString,
     ConfigMode: (schema.SchemaConfigMode) 0,
     Required: (bool) false,
     Optional: (bool) false,
     Computed: (bool) false,
     ForceNew: (bool) false,
     DiffSuppressFunc: (schema.SchemaDiffSuppressFunc) <nil>,
     DiffSuppressOnRefresh: (bool) false,
     Default: (interface {}) <nil>,
     DefaultFunc: (schema.SchemaDefaultFunc) <nil>,
     Description: (string) "",
     InputDefault: (string) "",
     StateFunc: (schema.SchemaStateFunc) <nil>,
     Elem: (interface {}) <nil>,
     MaxItems: (int) 0,
     MinItems: (int) 0,
     Set: (schema.SchemaSetFunc) <nil>,
     ComputedWhen: ([]string) <nil>,
     ConflictsWith: ([]string) <nil>,
     ExactlyOneOf: ([]string) <nil>,
     AtLeastOneOf: ([]string) <nil>,
     RequiredWith: ([]string) <nil>,
     Deprecated: (string) "",
     ValidateFunc: (schema.SchemaValidateFunc) <nil>,
     ValidateDiagFunc: (schema.SchemaValidateDiagFunc) <nil>,
     Sensitive: (bool) false
    }),
    (string) (len=3) "bar": (*schema.Schema)({
     Type: (schema.ValueType) TypeInt,
     ConfigMode: (schema.SchemaConfigMode) 0,
     Required: (bool) false,
     Optional: (bool) false,
     Computed: (bool) false,
     ForceNew: (bool) false,
     DiffSuppressFunc: (schema.SchemaDiffSuppressFunc) <nil>,
     DiffSuppressOnRefresh: (bool) false,
     Default: (interface {}) <nil>,
     DefaultFunc: (schema.SchemaDefaultFunc) <nil>,
     Description: (string) "",
     InputDefault: (string) "",
     StateFunc: (schema.SchemaStateFunc) <nil>,
     Elem: (interface {}) <nil>,
     MaxItems: (int) 0,
     MinItems: (int) 0,
     Set: (schema.SchemaSetFunc) <nil>,
     ComputedWhen: ([]string) <nil>,
     ConflictsWith: ([]string) <nil>,
     ExactlyOneOf: ([]string) <nil>,
     AtLeastOneOf: ([]string) <nil>,
     RequiredWith: ([]string) <nil>,
     Deprecated: (string) "",
     ValidateFunc: (schema.SchemaValidateFunc) <nil>,
     ValidateDiagFunc: (schema.SchemaValidateDiagFunc) <nil>,
     Sensitive: (bool) false
    })
   },
   SchemaFunc: (func() map[string]*schema.Schema) <nil>,
   SchemaVersion: (int) 0,
   MigrateState: (schema.StateMigrateFunc) <nil>,
   StateUpgraders: ([]schema.StateUpgrader) <nil>,
   Create: (schema.CreateFunc) <nil>,
   Read: (schema.ReadFunc) <nil>,
   Update: (schema.UpdateFunc) <nil>,
   Delete: (schema.DeleteFunc) <nil>,
   Exists: (schema.ExistsFunc) <nil>,
   CreateContext: (schema.CreateContextFunc) <nil>,
   ReadContext: (schema.ReadContextFunc) <nil>,
   UpdateContext: (schema.UpdateContextFunc) <nil>,
   DeleteContext: (schema.DeleteContextFunc) <nil>,
   CreateWithoutTimeout: (schema.CreateContextFunc) <nil>,
   ReadWithoutTimeout: (schema.ReadContextFunc) <nil>,
   UpdateWithoutTimeout: (schema.UpdateContextFunc) <nil>,
   DeleteWithoutTimeout: (schema.DeleteContextFunc) <nil>,
   CustomizeDiff: (schema.CustomizeDiffFunc) <nil>,
   Importer: (*schema.ResourceImporter)(<nil>),
   DeprecationMessage: (string) "",
   Timeouts: (*schema.ResourceTimeout)(<nil>),
   Description: (string) "",
   UseJSONNumber: (bool) false,
   EnableLegacyTypeSystemApplyErrors: (bool) false,
   EnableLegacyTypeSystemPlanErrors: (bool) false
  })
 },
 ProviderMetaSchema: (map[string]*schema.Schema) <nil>,
 ConfigureFunc: (schema.ConfigureFunc) <nil>,
 ConfigureContextFunc: (schema.ConfigureContextFunc) <nil>,
 configured: (bool) false,
 meta: (interface {}) <nil>,
 TerraformVersion: (string) ""
})
`).Equal(t, prov.DetailedSchemaDump())
}
