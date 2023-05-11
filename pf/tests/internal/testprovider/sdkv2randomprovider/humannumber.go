// Copyright 2016-2022, Pulumi Corporation.
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

package sdkv2randomprovider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func humanNumber() *schema.Resource {
	return &schema.Resource{
		Description: "A human generated random number.",

		CreateContext: humanNumberCreate,
		ReadContext:   humanNumberRead,
		UpdateContext: humanNumberUpdate,
		DeleteContext: humanNumberDelete,

		Schema: map[string]*schema.Schema{
			"suggestion": {
				// This description is used by the documentation generator and the language server.
				Description: "What number you think I should say. This will make it less random.",
				Type:        schema.TypeInt,
				Optional:    true,
			},
			"number": {
				Description: "I promise its random. I rolled a d6 and everything.",
				Type:        schema.TypeFloat,
				Computed:    true,
			},
			"fair": {
				Description: "If I actually chose randomly.",
				Type:        schema.TypeBool,
				Computed:    true,
			},
			"suggestion_updated": {
				Description: "If the suggestion was changed after the number was provided",
				Type:        schema.TypeBool,
				Computed:    true,
			},
		},
	}
}

func humanNumberCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	suggestion, ok := d.GetOk("suggestion")
	number := 4
	if ok {
		number = suggestion.(int)
	}

	if err := d.Set("number", number); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("suggestion_updated", false); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("fair", !ok); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d", number))

	return nil
}

func humanNumberRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	return nil
}

func humanNumberUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	if d.HasChange("suggestion") {
		err := d.Set("suggestion", d.Get("suggestion"))
		if err != nil {
			return diag.FromErr(err)
		}
		err = d.Set("suggestion_updated", true)
		if err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func humanNumberDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	return nil
}
