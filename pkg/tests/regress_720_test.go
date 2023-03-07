// Copyright 2016-2023, Pulumi Corporation.
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

package tests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	pluginsdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestRegress720(t *testing.T) {
	ctx := context.Background()

	resource := &schema.Resource{
		Schema: map[string]*pluginsdk.Schema{
			"storage_account_id": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
				//ValidateFunc: azure.ValidateResourceID,
			},

			"rule": {
				Type:     pluginsdk.TypeList,
				Optional: true,
				MinItems: 1,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"name": {
							Type:     pluginsdk.TypeString,
							Required: true,
							//ValidateFunc: validation.StringIsNotEmpty,
						},
						"enabled": {
							Type:     pluginsdk.TypeBool,
							Required: true,
						},
						"filters": {
							Type:     pluginsdk.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &pluginsdk.Resource{
								Schema: map[string]*pluginsdk.Schema{
									"blob_types": {
										Type:     pluginsdk.TypeSet,
										Required: true,
										Elem: &pluginsdk.Schema{
											Type: pluginsdk.TypeString,
											ValidateFunc: validation.StringInSlice([]string{
												"blockBlob",
												"appendBlob",
											}, false),
										},
										Set: pluginsdk.HashString,
									},
									"prefix_match": {
										Type:     pluginsdk.TypeSet,
										Optional: true,
										Elem:     &pluginsdk.Schema{Type: pluginsdk.TypeString},
										Set:      pluginsdk.HashString,
									},
									"match_blob_index_tag": {
										Type:     pluginsdk.TypeSet,
										Optional: true,
										Elem: &pluginsdk.Resource{
											Schema: map[string]*pluginsdk.Schema{
												"name": {
													Type:     pluginsdk.TypeString,
													Required: true,
													//ValidateFunc: validate.StorageBlobIndexTagName,
												},

												"operation": {
													Type:     pluginsdk.TypeString,
													Optional: true,
													ValidateFunc: validation.StringInSlice([]string{
														"==",
													}, false),
													Default: "==",
												},

												"value": {
													Type:     pluginsdk.TypeString,
													Required: true,
													//ValidateFunc: validate.StorageBlobIndexTagValue,
												},
											},
										},
									},
								},
							},
						},
						// lintignore:XS003
						"actions": {
							Type:     pluginsdk.TypeList,
							Required: true,
							MaxItems: 1,
							Elem: &pluginsdk.Resource{
								Schema: map[string]*pluginsdk.Schema{

									// lintignore:XS003
									"base_blob": {
										Type:     pluginsdk.TypeList,
										Optional: true,
										MaxItems: 1,
										Elem: &pluginsdk.Resource{
											Schema: map[string]*pluginsdk.Schema{
												"tier_to_cool_after_days_since_modification_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"tier_to_cool_after_days_since_last_access_time_greater_than": {
													Type:     pluginsdk.TypeInt,
													Optional: true,
													Default:  -1,
												},
												"tier_to_cool_after_days_since_creation_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"tier_to_archive_after_days_since_modification_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"tier_to_archive_after_days_since_last_access_time_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"tier_to_archive_after_days_since_last_tier_change_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"tier_to_archive_after_days_since_creation_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"delete_after_days_since_modification_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"delete_after_days_since_last_access_time_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"delete_after_days_since_creation_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
											},
										},
									},

									// lintignore:XS003
									"snapshot": {
										Type:     pluginsdk.TypeList,
										Optional: true,
										MaxItems: 1,
										Elem: &pluginsdk.Resource{
											Schema: map[string]*pluginsdk.Schema{
												"change_tier_to_archive_after_days_since_creation": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"tier_to_archive_after_days_since_last_tier_change_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"change_tier_to_cool_after_days_since_creation": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"delete_after_days_since_creation_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
											},
										},
									},

									"version": {
										Type:     pluginsdk.TypeList,
										Optional: true,
										MaxItems: 1,
										Elem: &pluginsdk.Resource{
											Schema: map[string]*pluginsdk.Schema{
												"change_tier_to_archive_after_days_since_creation": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"tier_to_archive_after_days_since_last_tier_change_greater_than": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"change_tier_to_cool_after_days_since_creation": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
												"delete_after_days_since_creation": {
													Type:         pluginsdk.TypeInt,
													Optional:     true,
													Default:      -1,
													ValidateFunc: validation.IntBetween(0, 99999),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tfProvider := &schema.Provider{
		Schema: map[string]*schema.Schema{},
		ResourcesMap: map[string]*schema.Resource{
			"azure_storage_management_policy": resource,
		},
	}

	p := shimv2.NewProvider(tfProvider, shimv2.WithDiffStrategy(shimv2.PlanState))

	info := tfbridge.ProviderInfo{
		P:           p,
		Name:        "azure",
		Description: "..",
		Version:     "0.0.1",
		Resources: map[string]*tfbridge.ResourceInfo{
			"azure_storage_management_policy": {Tok: "azure:storage/managementPolicy:ManagementPolicy"},
		},
	}

	server := tfbridge.NewProvider(ctx,
		nil,      /* hostClient */
		"aws",    /* module */
		"",       /* version */
		p,        /* tf */
		info,     /* info */
		[]byte{}, /* pulumiSchema */
	)

	checkCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Check",
	  "request": {
	    "urn": "urn:pulumi:dev::repro-pulumi-terraform-bridge-720::azure:storage/managementPolicy:ManagementPolicy::managementPolicyRobotFramework",
	    "olds": {},
	    "news": {
	      "rules": [
	        {
	          "actions": {
	            "baseBlob": {
	              "deleteAfterDaysSinceModificationGreaterThan": 30
	            },
	            "snapshot": {
	              "deleteAfterDaysSinceCreationGreaterThan": 30
	            },
	            "version": {
	              "deleteAfterDaysSinceCreation": 30
	            }
	          },
	          "enabled": true,
	          "filters": {
	            "blobTypes": [
	              "blockBlob"
	            ],
	            "prefixMatches": [
	              "my-container"
	            ]
	          },
	          "name": "robot-framework"
	        }
	      ],
	      "storageAccountId": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	    },
	    "randomSeed": "4Cj0wILBnEZaBXGvBqCIF+ErKursPH/WtmauwGkUa6I="
	  },
	  "response": {
	    "inputs": {
	      "__defaults": [],
	      "rules": [
	        {
	          "__defaults": [],
	          "actions": {
	            "__defaults": [],
	            "baseBlob": {
	              "__defaults": [
	                "tierToCoolAfterDaysSinceLastAccessTimeGreaterThan"
	              ],
	              "deleteAfterDaysSinceModificationGreaterThan": 30,
	              "tierToCoolAfterDaysSinceLastAccessTimeGreaterThan": -1
	            },
	            "snapshot": {
	              "__defaults": [],
	              "deleteAfterDaysSinceCreationGreaterThan": 30
	            },
	            "version": {
	              "__defaults": [],
	              "deleteAfterDaysSinceCreation": 30
	            }
	          },
	          "enabled": true,
	          "filters": {
	            "__defaults": [],
	            "blobTypes": [
	              "blockBlob"
	            ],
	            "prefixMatches": [
	              "my-container"
	            ]
	          },
	          "name": "robot-framework"
	        }
	      ],
	      "storageAccountId": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	    }
	  }
	}
	`

	t.Run("check", func(t *testing.T) {
		testutils.Replay(t, server, checkCase)
	})

	createTest := `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::repro-pulumi-terraform-bridge-720::azure:storage/managementPolicy:ManagementPolicy::managementPolicyRobotFramework",
	    "properties": {
	      "__defaults": [],
	      "rules": [
		{
		  "__defaults": [],
		  "actions": {
		    "__defaults": [],
		    "baseBlob": {
		      "__defaults": [
			"tierToCoolAfterDaysSinceLastAccessTimeGreaterThan"
		      ],
		      "deleteAfterDaysSinceModificationGreaterThan": 30,
		      "tierToCoolAfterDaysSinceLastAccessTimeGreaterThan": -1
		    },
		    "snapshot": {
		      "__defaults": [],
		      "deleteAfterDaysSinceCreationGreaterThan": 30
		    },
		    "version": {
		      "__defaults": [],
		      "deleteAfterDaysSinceCreation": 30
		    }
		  },
		  "enabled": true,
		  "filters": {
		    "__defaults": [],
		    "blobTypes": [
		      "blockBlob"
		    ],
		    "prefixMatches": [
		      "my-container"
		    ]
		  },
		  "name": "robot-framework"
		}
	      ],
	      "storageAccountId": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	    },
	    "preview": true
	  },
	  "response": {
	    "properties": {
	      "id": "",
	      "rules": [
		{
		  "actions": {
		    "baseBlob": {
		      "deleteAfterDaysSinceCreationGreaterThan": -1,
		      "deleteAfterDaysSinceLastAccessTimeGreaterThan": -1,
		      "deleteAfterDaysSinceModificationGreaterThan": 30,
		      "tierToArchiveAfterDaysSinceCreationGreaterThan": -1,
		      "tierToArchiveAfterDaysSinceLastAccessTimeGreaterThan": -1,
		      "tierToArchiveAfterDaysSinceLastTierChangeGreaterThan": -1,
		      "tierToArchiveAfterDaysSinceModificationGreaterThan": -1,
		      "tierToCoolAfterDaysSinceCreationGreaterThan": -1,
		      "tierToCoolAfterDaysSinceLastAccessTimeGreaterThan": -1,
		      "tierToCoolAfterDaysSinceModificationGreaterThan": -1
		    },
		    "snapshot": {
		      "changeTierToArchiveAfterDaysSinceCreation": -1,
		      "changeTierToCoolAfterDaysSinceCreation": -1,
		      "deleteAfterDaysSinceCreationGreaterThan": 30,
		      "tierToArchiveAfterDaysSinceLastTierChangeGreaterThan": -1
		    },
		    "version": {
		      "changeTierToArchiveAfterDaysSinceCreation": -1,
		      "changeTierToCoolAfterDaysSinceCreation": -1,
		      "deleteAfterDaysSinceCreation": 30,
		      "tierToArchiveAfterDaysSinceLastTierChangeGreaterThan": -1
		    }
		  },
		  "enabled": true,
		  "filters": {
		    "blobTypes": [
		      "blockBlob"
		    ],
		    "matchBlobIndexTags": [],
		    "prefixMatches": [
		      "my-container"
		    ]
		  },
		  "name": "robot-framework"
		}
	      ]
	    }
	  }
	}`

	t.Run("create", func(t *testing.T) {
		testutils.Replay(t, server, createTest)
	})
}
