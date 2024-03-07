Provides a Wavefront Dashboard JSON resource. This allows dashboards to be created, updated, and deleted.

## Example Usage

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as wavefront from "@pulumi/wavefront";

const testDashboardJson = new wavefront.DashboardJson("testDashboardJson", {dashboardJson: `  {
    "acl": {
      "canModify": [
        "group-uuid",
        "role-uuid"
      ],
      "canView": [
        "group-uuid",
        "role-uuid"
      ]
    },
    "name": "Terraform Test Dashboard Json",
    "description": "a",
    "eventFilterType": "BYCHART",
    "eventQuery": "",
    "defaultTimeWindow": "",
    "url": "tftestimport",
    "displayDescription": false,
    "displaySectionTableOfContents": true,
    "displayQueryParameters": false,
    "sections": [
      {
        "name": "section 1",
        "rows": [
          {
            "charts": [
              {
                "name": "chart 1",
                "sources": [
                  {
                    "name": "source 1",
                    "query": "ts()",
                    "scatterPlotSource": "Y",
                    "querybuilderEnabled": false,
                    "sourceDescription": ""
                  }
                ],
                "units": "someunit",
                "base": 0,
                "noDefaultEvents": false,
                "interpolatePoints": false,
                "includeObsoleteMetrics": false,
                "description": "This is chart 1, showing something",
                "chartSettings": {
                  "type": "markdown-widget",
                  "max": 100,
                  "expectedDataSpacing": 120,
                  "windowing": "full",
                  "windowSize": 10,
                  "autoColumnTags": false,
                  "columnTags": "deprecated",
                  "tagMode": "all",
                  "numTags": 2,
                  "customTags": [
                    "tag1",
                    "tag2"
                  ],
                  "groupBySource": true,
                  "y1Max": 100,
                  "y1Units": "units",
                  "y0ScaleSIBy1024": true,
                  "y1ScaleSIBy1024": true,
                  "y0UnitAutoscaling": true,
                  "y1UnitAutoscaling": true,
                  "fixedLegendEnabled": true,
                  "fixedLegendUseRawStats": true,
                  "fixedLegendPosition": "RIGHT",
                  "fixedLegendDisplayStats": [
                    "stat1",
                    "stat2"
                  ],
                  "fixedLegendFilterSort": "TOP",
                  "fixedLegendFilterLimit": 1,
                  "fixedLegendFilterField": "CURRENT",
                  "plainMarkdownContent": "markdown content"
                },
                "chartAttributes": {
                  "dashboardLinks": {
                    "*": {
                      "variables": {
                        "xxx": "xxx"
                      },
                      "destination": "/dashboards/xxxx"
                    }
                  }
                },
                "summarization": "MEAN"
              }
            ],
            "heightFactor": 50
          }
        ]
      }
    ],
    "parameterDetails": {
      "param": {
        "hideFromView": false,
        "description": null,
        "allowAll": null,
        "tagKey": null,
        "queryValue": null,
        "dynamicFieldType": null,
        "reverseDynSort": null,
        "parameterType": "SIMPLE",
        "label": "test",
        "defaultValue": "Label",
        "valuesToReadableStrings": {
          "Label": "test"
        },
        "selectedLabel": "Label",
        "value": "test"
      }
    },
    "tags": {
      "customerTags": [
        "terraform"
      ]
    }
  }

`});
```
```python
import pulumi
import pulumi_wavefront as wavefront

test_dashboard_json = wavefront.DashboardJson("testDashboardJson", dashboard_json="""  {
    "acl": {
      "canModify": [
        "group-uuid",
        "role-uuid"
      ],
      "canView": [
        "group-uuid",
        "role-uuid"
      ]
    },
    "name": "Terraform Test Dashboard Json",
    "description": "a",
    "eventFilterType": "BYCHART",
    "eventQuery": "",
    "defaultTimeWindow": "",
    "url": "tftestimport",
    "displayDescription": false,
    "displaySectionTableOfContents": true,
    "displayQueryParameters": false,
    "sections": [
      {
        "name": "section 1",
        "rows": [
          {
            "charts": [
              {
                "name": "chart 1",
                "sources": [
                  {
                    "name": "source 1",
                    "query": "ts()",
                    "scatterPlotSource": "Y",
                    "querybuilderEnabled": false,
                    "sourceDescription": ""
                  }
                ],
                "units": "someunit",
                "base": 0,
                "noDefaultEvents": false,
                "interpolatePoints": false,
                "includeObsoleteMetrics": false,
                "description": "This is chart 1, showing something",
                "chartSettings": {
                  "type": "markdown-widget",
                  "max": 100,
                  "expectedDataSpacing": 120,
                  "windowing": "full",
                  "windowSize": 10,
                  "autoColumnTags": false,
                  "columnTags": "deprecated",
                  "tagMode": "all",
                  "numTags": 2,
                  "customTags": [
                    "tag1",
                    "tag2"
                  ],
                  "groupBySource": true,
                  "y1Max": 100,
                  "y1Units": "units",
                  "y0ScaleSIBy1024": true,
                  "y1ScaleSIBy1024": true,
                  "y0UnitAutoscaling": true,
                  "y1UnitAutoscaling": true,
                  "fixedLegendEnabled": true,
                  "fixedLegendUseRawStats": true,
                  "fixedLegendPosition": "RIGHT",
                  "fixedLegendDisplayStats": [
                    "stat1",
                    "stat2"
                  ],
                  "fixedLegendFilterSort": "TOP",
                  "fixedLegendFilterLimit": 1,
                  "fixedLegendFilterField": "CURRENT",
                  "plainMarkdownContent": "markdown content"
                },
                "chartAttributes": {
                  "dashboardLinks": {
                    "*": {
                      "variables": {
                        "xxx": "xxx"
                      },
                      "destination": "/dashboards/xxxx"
                    }
                  }
                },
                "summarization": "MEAN"
              }
            ],
            "heightFactor": 50
          }
        ]
      }
    ],
    "parameterDetails": {
      "param": {
        "hideFromView": false,
        "description": null,
        "allowAll": null,
        "tagKey": null,
        "queryValue": null,
        "dynamicFieldType": null,
        "reverseDynSort": null,
        "parameterType": "SIMPLE",
        "label": "test",
        "defaultValue": "Label",
        "valuesToReadableStrings": {
          "Label": "test"
        },
        "selectedLabel": "Label",
        "value": "test"
      }
    },
    "tags": {
      "customerTags": [
        "terraform"
      ]
    }
  }

""")
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Wavefront = Pulumi.Wavefront;

return await Deployment.RunAsync(() => 
{
    var testDashboardJson = new Wavefront.DashboardJson("testDashboardJson", new()
    {
        JSON = @"  {
    ""acl"": {
      ""canModify"": [
        ""group-uuid"",
        ""role-uuid""
      ],
      ""canView"": [
        ""group-uuid"",
        ""role-uuid""
      ]
    },
    ""name"": ""Terraform Test Dashboard Json"",
    ""description"": ""a"",
    ""eventFilterType"": ""BYCHART"",
    ""eventQuery"": """",
    ""defaultTimeWindow"": """",
    ""url"": ""tftestimport"",
    ""displayDescription"": false,
    ""displaySectionTableOfContents"": true,
    ""displayQueryParameters"": false,
    ""sections"": [
      {
        ""name"": ""section 1"",
        ""rows"": [
          {
            ""charts"": [
              {
                ""name"": ""chart 1"",
                ""sources"": [
                  {
                    ""name"": ""source 1"",
                    ""query"": ""ts()"",
                    ""scatterPlotSource"": ""Y"",
                    ""querybuilderEnabled"": false,
                    ""sourceDescription"": """"
                  }
                ],
                ""units"": ""someunit"",
                ""base"": 0,
                ""noDefaultEvents"": false,
                ""interpolatePoints"": false,
                ""includeObsoleteMetrics"": false,
                ""description"": ""This is chart 1, showing something"",
                ""chartSettings"": {
                  ""type"": ""markdown-widget"",
                  ""max"": 100,
                  ""expectedDataSpacing"": 120,
                  ""windowing"": ""full"",
                  ""windowSize"": 10,
                  ""autoColumnTags"": false,
                  ""columnTags"": ""deprecated"",
                  ""tagMode"": ""all"",
                  ""numTags"": 2,
                  ""customTags"": [
                    ""tag1"",
                    ""tag2""
                  ],
                  ""groupBySource"": true,
                  ""y1Max"": 100,
                  ""y1Units"": ""units"",
                  ""y0ScaleSIBy1024"": true,
                  ""y1ScaleSIBy1024"": true,
                  ""y0UnitAutoscaling"": true,
                  ""y1UnitAutoscaling"": true,
                  ""fixedLegendEnabled"": true,
                  ""fixedLegendUseRawStats"": true,
                  ""fixedLegendPosition"": ""RIGHT"",
                  ""fixedLegendDisplayStats"": [
                    ""stat1"",
                    ""stat2""
                  ],
                  ""fixedLegendFilterSort"": ""TOP"",
                  ""fixedLegendFilterLimit"": 1,
                  ""fixedLegendFilterField"": ""CURRENT"",
                  ""plainMarkdownContent"": ""markdown content""
                },
                ""chartAttributes"": {
                  ""dashboardLinks"": {
                    ""*"": {
                      ""variables"": {
                        ""xxx"": ""xxx""
                      },
                      ""destination"": ""/dashboards/xxxx""
                    }
                  }
                },
                ""summarization"": ""MEAN""
              }
            ],
            ""heightFactor"": 50
          }
        ]
      }
    ],
    ""parameterDetails"": {
      ""param"": {
        ""hideFromView"": false,
        ""description"": null,
        ""allowAll"": null,
        ""tagKey"": null,
        ""queryValue"": null,
        ""dynamicFieldType"": null,
        ""reverseDynSort"": null,
        ""parameterType"": ""SIMPLE"",
        ""label"": ""test"",
        ""defaultValue"": ""Label"",
        ""valuesToReadableStrings"": {
          ""Label"": ""test""
        },
        ""selectedLabel"": ""Label"",
        ""value"": ""test""
      }
    },
    ""tags"": {
      ""customerTags"": [
        ""terraform""
      ]
    }
  }

",
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi-wavefront/sdk/v3/go/wavefront"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := wavefront.NewDashboardJson(ctx, "testDashboardJson", &wavefront.DashboardJsonArgs{
			DashboardJson: pulumi.String(`  {
    "acl": {
      "canModify": [
        "group-uuid",
        "role-uuid"
      ],
      "canView": [
        "group-uuid",
        "role-uuid"
      ]
    },
    "name": "Terraform Test Dashboard Json",
    "description": "a",
    "eventFilterType": "BYCHART",
    "eventQuery": "",
    "defaultTimeWindow": "",
    "url": "tftestimport",
    "displayDescription": false,
    "displaySectionTableOfContents": true,
    "displayQueryParameters": false,
    "sections": [
      {
        "name": "section 1",
        "rows": [
          {
            "charts": [
              {
                "name": "chart 1",
                "sources": [
                  {
                    "name": "source 1",
                    "query": "ts()",
                    "scatterPlotSource": "Y",
                    "querybuilderEnabled": false,
                    "sourceDescription": ""
                  }
                ],
                "units": "someunit",
                "base": 0,
                "noDefaultEvents": false,
                "interpolatePoints": false,
                "includeObsoleteMetrics": false,
                "description": "This is chart 1, showing something",
                "chartSettings": {
                  "type": "markdown-widget",
                  "max": 100,
                  "expectedDataSpacing": 120,
                  "windowing": "full",
                  "windowSize": 10,
                  "autoColumnTags": false,
                  "columnTags": "deprecated",
                  "tagMode": "all",
                  "numTags": 2,
                  "customTags": [
                    "tag1",
                    "tag2"
                  ],
                  "groupBySource": true,
                  "y1Max": 100,
                  "y1Units": "units",
                  "y0ScaleSIBy1024": true,
                  "y1ScaleSIBy1024": true,
                  "y0UnitAutoscaling": true,
                  "y1UnitAutoscaling": true,
                  "fixedLegendEnabled": true,
                  "fixedLegendUseRawStats": true,
                  "fixedLegendPosition": "RIGHT",
                  "fixedLegendDisplayStats": [
                    "stat1",
                    "stat2"
                  ],
                  "fixedLegendFilterSort": "TOP",
                  "fixedLegendFilterLimit": 1,
                  "fixedLegendFilterField": "CURRENT",
                  "plainMarkdownContent": "markdown content"
                },
                "chartAttributes": {
                  "dashboardLinks": {
                    "*": {
                      "variables": {
                        "xxx": "xxx"
                      },
                      "destination": "/dashboards/xxxx"
                    }
                  }
                },
                "summarization": "MEAN"
              }
            ],
            "heightFactor": 50
          }
        ]
      }
    ],
    "parameterDetails": {
      "param": {
        "hideFromView": false,
        "description": null,
        "allowAll": null,
        "tagKey": null,
        "queryValue": null,
        "dynamicFieldType": null,
        "reverseDynSort": null,
        "parameterType": "SIMPLE",
        "label": "test",
        "defaultValue": "Label",
        "valuesToReadableStrings": {
          "Label": "test"
        },
        "selectedLabel": "Label",
        "value": "test"
      }
    },
    "tags": {
      "customerTags": [
        "terraform"
      ]
    }
  }

`),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.wavefront.DashboardJson;
import com.pulumi.wavefront.DashboardJsonArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var testDashboardJson = new DashboardJson("testDashboardJson", DashboardJsonArgs.builder()        
            .dashboardJson("""
  {
    "acl": {
      "canModify": [
        "group-uuid",
        "role-uuid"
      ],
      "canView": [
        "group-uuid",
        "role-uuid"
      ]
    },
    "name": "Terraform Test Dashboard Json",
    "description": "a",
    "eventFilterType": "BYCHART",
    "eventQuery": "",
    "defaultTimeWindow": "",
    "url": "tftestimport",
    "displayDescription": false,
    "displaySectionTableOfContents": true,
    "displayQueryParameters": false,
    "sections": [
      {
        "name": "section 1",
        "rows": [
          {
            "charts": [
              {
                "name": "chart 1",
                "sources": [
                  {
                    "name": "source 1",
                    "query": "ts()",
                    "scatterPlotSource": "Y",
                    "querybuilderEnabled": false,
                    "sourceDescription": ""
                  }
                ],
                "units": "someunit",
                "base": 0,
                "noDefaultEvents": false,
                "interpolatePoints": false,
                "includeObsoleteMetrics": false,
                "description": "This is chart 1, showing something",
                "chartSettings": {
                  "type": "markdown-widget",
                  "max": 100,
                  "expectedDataSpacing": 120,
                  "windowing": "full",
                  "windowSize": 10,
                  "autoColumnTags": false,
                  "columnTags": "deprecated",
                  "tagMode": "all",
                  "numTags": 2,
                  "customTags": [
                    "tag1",
                    "tag2"
                  ],
                  "groupBySource": true,
                  "y1Max": 100,
                  "y1Units": "units",
                  "y0ScaleSIBy1024": true,
                  "y1ScaleSIBy1024": true,
                  "y0UnitAutoscaling": true,
                  "y1UnitAutoscaling": true,
                  "fixedLegendEnabled": true,
                  "fixedLegendUseRawStats": true,
                  "fixedLegendPosition": "RIGHT",
                  "fixedLegendDisplayStats": [
                    "stat1",
                    "stat2"
                  ],
                  "fixedLegendFilterSort": "TOP",
                  "fixedLegendFilterLimit": 1,
                  "fixedLegendFilterField": "CURRENT",
                  "plainMarkdownContent": "markdown content"
                },
                "chartAttributes": {
                  "dashboardLinks": {
                    "*": {
                      "variables": {
                        "xxx": "xxx"
                      },
                      "destination": "/dashboards/xxxx"
                    }
                  }
                },
                "summarization": "MEAN"
              }
            ],
            "heightFactor": 50
          }
        ]
      }
    ],
    "parameterDetails": {
      "param": {
        "hideFromView": false,
        "description": null,
        "allowAll": null,
        "tagKey": null,
        "queryValue": null,
        "dynamicFieldType": null,
        "reverseDynSort": null,
        "parameterType": "SIMPLE",
        "label": "test",
        "defaultValue": "Label",
        "valuesToReadableStrings": {
          "Label": "test"
        },
        "selectedLabel": "Label",
        "value": "test"
      }
    },
    "tags": {
      "customerTags": [
        "terraform"
      ]
    }
  }

            """)
            .build());

    }
}
```
```yaml
resources:
  testDashboardJson:
    type: wavefront:DashboardJson
    properties:
      dashboardJson: |2+
          {
            "acl": {
              "canModify": [
                "group-uuid",
                "role-uuid"
              ],
              "canView": [
                "group-uuid",
                "role-uuid"
              ]
            },
            "name": "Terraform Test Dashboard Json",
            "description": "a",
            "eventFilterType": "BYCHART",
            "eventQuery": "",
            "defaultTimeWindow": "",
            "url": "tftestimport",
            "displayDescription": false,
            "displaySectionTableOfContents": true,
            "displayQueryParameters": false,
            "sections": [
              {
                "name": "section 1",
                "rows": [
                  {
                    "charts": [
                      {
                        "name": "chart 1",
                        "sources": [
                          {
                            "name": "source 1",
                            "query": "ts()",
                            "scatterPlotSource": "Y",
                            "querybuilderEnabled": false,
                            "sourceDescription": ""
                          }
                        ],
                        "units": "someunit",
                        "base": 0,
                        "noDefaultEvents": false,
                        "interpolatePoints": false,
                        "includeObsoleteMetrics": false,
                        "description": "This is chart 1, showing something",
                        "chartSettings": {
                          "type": "markdown-widget",
                          "max": 100,
                          "expectedDataSpacing": 120,
                          "windowing": "full",
                          "windowSize": 10,
                          "autoColumnTags": false,
                          "columnTags": "deprecated",
                          "tagMode": "all",
                          "numTags": 2,
                          "customTags": [
                            "tag1",
                            "tag2"
                          ],
                          "groupBySource": true,
                          "y1Max": 100,
                          "y1Units": "units",
                          "y0ScaleSIBy1024": true,
                          "y1ScaleSIBy1024": true,
                          "y0UnitAutoscaling": true,
                          "y1UnitAutoscaling": true,
                          "fixedLegendEnabled": true,
                          "fixedLegendUseRawStats": true,
                          "fixedLegendPosition": "RIGHT",
                          "fixedLegendDisplayStats": [
                            "stat1",
                            "stat2"
                          ],
                          "fixedLegendFilterSort": "TOP",
                          "fixedLegendFilterLimit": 1,
                          "fixedLegendFilterField": "CURRENT",
                          "plainMarkdownContent": "markdown content"
                        },
                        "chartAttributes": {
                          "dashboardLinks": {
                            "*": {
                              "variables": {
                                "xxx": "xxx"
                              },
                              "destination": "/dashboards/xxxx"
                            }
                          }
                        },
                        "summarization": "MEAN"
                      }
                    ],
                    "heightFactor": 50
                  }
                ]
              }
            ],
            "parameterDetails": {
              "param": {
                "hideFromView": false,
                "description": null,
                "allowAll": null,
                "tagKey": null,
                "queryValue": null,
                "dynamicFieldType": null,
                "reverseDynSort": null,
                "parameterType": "SIMPLE",
                "label": "test",
                "defaultValue": "Label",
                "valuesToReadableStrings": {
                  "Label": "test"
                },
                "selectedLabel": "Label",
                "value": "test"
              }
            },
            "tags": {
              "customerTags": [
                "terraform"
              ]
            }
          }
```
<!--End PulumiCodeChooser -->

*
*Note:
** If there are dynamic variables in the Wavefront dashboard json, then these variables must be present in a separate file as mentioned in the section below.

## Import

Dashboard JSON can be imported by using the `id`, e.g.:

```sh
$ pulumi import wavefront:index/dashboardJson:DashboardJson dashboard_json tftestimport
```
