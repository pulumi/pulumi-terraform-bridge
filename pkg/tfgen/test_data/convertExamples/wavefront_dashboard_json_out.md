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
```protobuf
CqQcCqEcChF0ZXN0RGFzaGJvYXJkSnNvbhIRdGVzdERhc2hib2FyZEpzb24aK3dhdmVmcm9udDppbmRleC9kYXNoYm9hcmRKc29uOkRhc2hib2FyZEpzb24iyxsKDWRhc2hib2FyZEpzb24SuRsSthsKsxsKsBsSrRsgIHsKICAgICJhY2wiOiB7CiAgICAgICJjYW5Nb2RpZnkiOiBbCiAgICAgICAgImdyb3VwLXV1aWQiLAogICAgICAgICJyb2xlLXV1aWQiCiAgICAgIF0sCiAgICAgICJjYW5WaWV3IjogWwogICAgICAgICJncm91cC11dWlkIiwKICAgICAgICAicm9sZS11dWlkIgogICAgICBdCiAgICB9LAogICAgIm5hbWUiOiAiVGVycmFmb3JtIFRlc3QgRGFzaGJvYXJkIEpzb24iLAogICAgImRlc2NyaXB0aW9uIjogImEiLAogICAgImV2ZW50RmlsdGVyVHlwZSI6ICJCWUNIQVJUIiwKICAgICJldmVudFF1ZXJ5IjogIiIsCiAgICAiZGVmYXVsdFRpbWVXaW5kb3ciOiAiIiwKICAgICJ1cmwiOiAidGZ0ZXN0aW1wb3J0IiwKICAgICJkaXNwbGF5RGVzY3JpcHRpb24iOiBmYWxzZSwKICAgICJkaXNwbGF5U2VjdGlvblRhYmxlT2ZDb250ZW50cyI6IHRydWUsCiAgICAiZGlzcGxheVF1ZXJ5UGFyYW1ldGVycyI6IGZhbHNlLAogICAgInNlY3Rpb25zIjogWwogICAgICB7CiAgICAgICAgIm5hbWUiOiAic2VjdGlvbiAxIiwKICAgICAgICAicm93cyI6IFsKICAgICAgICAgIHsKICAgICAgICAgICAgImNoYXJ0cyI6IFsKICAgICAgICAgICAgICB7CiAgICAgICAgICAgICAgICAibmFtZSI6ICJjaGFydCAxIiwKICAgICAgICAgICAgICAgICJzb3VyY2VzIjogWwogICAgICAgICAgICAgICAgICB7CiAgICAgICAgICAgICAgICAgICAgIm5hbWUiOiAic291cmNlIDEiLAogICAgICAgICAgICAgICAgICAgICJxdWVyeSI6ICJ0cygpIiwKICAgICAgICAgICAgICAgICAgICAic2NhdHRlclBsb3RTb3VyY2UiOiAiWSIsCiAgICAgICAgICAgICAgICAgICAgInF1ZXJ5YnVpbGRlckVuYWJsZWQiOiBmYWxzZSwKICAgICAgICAgICAgICAgICAgICAic291cmNlRGVzY3JpcHRpb24iOiAiIgogICAgICAgICAgICAgICAgICB9CiAgICAgICAgICAgICAgICBdLAogICAgICAgICAgICAgICAgInVuaXRzIjogInNvbWV1bml0IiwKICAgICAgICAgICAgICAgICJiYXNlIjogMCwKICAgICAgICAgICAgICAgICJub0RlZmF1bHRFdmVudHMiOiBmYWxzZSwKICAgICAgICAgICAgICAgICJpbnRlcnBvbGF0ZVBvaW50cyI6IGZhbHNlLAogICAgICAgICAgICAgICAgImluY2x1ZGVPYnNvbGV0ZU1ldHJpY3MiOiBmYWxzZSwKICAgICAgICAgICAgICAgICJkZXNjcmlwdGlvbiI6ICJUaGlzIGlzIGNoYXJ0IDEsIHNob3dpbmcgc29tZXRoaW5nIiwKICAgICAgICAgICAgICAgICJjaGFydFNldHRpbmdzIjogewogICAgICAgICAgICAgICAgICAidHlwZSI6ICJtYXJrZG93bi13aWRnZXQiLAogICAgICAgICAgICAgICAgICAibWF4IjogMTAwLAogICAgICAgICAgICAgICAgICAiZXhwZWN0ZWREYXRhU3BhY2luZyI6IDEyMCwKICAgICAgICAgICAgICAgICAgIndpbmRvd2luZyI6ICJmdWxsIiwKICAgICAgICAgICAgICAgICAgIndpbmRvd1NpemUiOiAxMCwKICAgICAgICAgICAgICAgICAgImF1dG9Db2x1bW5UYWdzIjogZmFsc2UsCiAgICAgICAgICAgICAgICAgICJjb2x1bW5UYWdzIjogImRlcHJlY2F0ZWQiLAogICAgICAgICAgICAgICAgICAidGFnTW9kZSI6ICJhbGwiLAogICAgICAgICAgICAgICAgICAibnVtVGFncyI6IDIsCiAgICAgICAgICAgICAgICAgICJjdXN0b21UYWdzIjogWwogICAgICAgICAgICAgICAgICAgICJ0YWcxIiwKICAgICAgICAgICAgICAgICAgICAidGFnMiIKICAgICAgICAgICAgICAgICAgXSwKICAgICAgICAgICAgICAgICAgImdyb3VwQnlTb3VyY2UiOiB0cnVlLAogICAgICAgICAgICAgICAgICAieTFNYXgiOiAxMDAsCiAgICAgICAgICAgICAgICAgICJ5MVVuaXRzIjogInVuaXRzIiwKICAgICAgICAgICAgICAgICAgInkwU2NhbGVTSUJ5MTAyNCI6IHRydWUsCiAgICAgICAgICAgICAgICAgICJ5MVNjYWxlU0lCeTEwMjQiOiB0cnVlLAogICAgICAgICAgICAgICAgICAieTBVbml0QXV0b3NjYWxpbmciOiB0cnVlLAogICAgICAgICAgICAgICAgICAieTFVbml0QXV0b3NjYWxpbmciOiB0cnVlLAogICAgICAgICAgICAgICAgICAiZml4ZWRMZWdlbmRFbmFibGVkIjogdHJ1ZSwKICAgICAgICAgICAgICAgICAgImZpeGVkTGVnZW5kVXNlUmF3U3RhdHMiOiB0cnVlLAogICAgICAgICAgICAgICAgICAiZml4ZWRMZWdlbmRQb3NpdGlvbiI6ICJSSUdIVCIsCiAgICAgICAgICAgICAgICAgICJmaXhlZExlZ2VuZERpc3BsYXlTdGF0cyI6IFsKICAgICAgICAgICAgICAgICAgICAic3RhdDEiLAogICAgICAgICAgICAgICAgICAgICJzdGF0MiIKICAgICAgICAgICAgICAgICAgXSwKICAgICAgICAgICAgICAgICAgImZpeGVkTGVnZW5kRmlsdGVyU29ydCI6ICJUT1AiLAogICAgICAgICAgICAgICAgICAiZml4ZWRMZWdlbmRGaWx0ZXJMaW1pdCI6IDEsCiAgICAgICAgICAgICAgICAgICJmaXhlZExlZ2VuZEZpbHRlckZpZWxkIjogIkNVUlJFTlQiLAogICAgICAgICAgICAgICAgICAicGxhaW5NYXJrZG93bkNvbnRlbnQiOiAibWFya2Rvd24gY29udGVudCIKICAgICAgICAgICAgICAgIH0sCiAgICAgICAgICAgICAgICAiY2hhcnRBdHRyaWJ1dGVzIjogewogICAgICAgICAgICAgICAgICAiZGFzaGJvYXJkTGlua3MiOiB7CiAgICAgICAgICAgICAgICAgICAgIioiOiB7CiAgICAgICAgICAgICAgICAgICAgICAidmFyaWFibGVzIjogewogICAgICAgICAgICAgICAgICAgICAgICAieHh4IjogInh4eCIKICAgICAgICAgICAgICAgICAgICAgIH0sCiAgICAgICAgICAgICAgICAgICAgICAiZGVzdGluYXRpb24iOiAiL2Rhc2hib2FyZHMveHh4eCIKICAgICAgICAgICAgICAgICAgICB9CiAgICAgICAgICAgICAgICAgIH0KICAgICAgICAgICAgICAgIH0sCiAgICAgICAgICAgICAgICAic3VtbWFyaXphdGlvbiI6ICJNRUFOIgogICAgICAgICAgICAgIH0KICAgICAgICAgICAgXSwKICAgICAgICAgICAgImhlaWdodEZhY3RvciI6IDUwCiAgICAgICAgICB9CiAgICAgICAgXQogICAgICB9CiAgICBdLAogICAgInBhcmFtZXRlckRldGFpbHMiOiB7CiAgICAgICJwYXJhbSI6IHsKICAgICAgICAiaGlkZUZyb21WaWV3IjogZmFsc2UsCiAgICAgICAgImRlc2NyaXB0aW9uIjogbnVsbCwKICAgICAgICAiYWxsb3dBbGwiOiBudWxsLAogICAgICAgICJ0YWdLZXkiOiBudWxsLAogICAgICAgICJxdWVyeVZhbHVlIjogbnVsbCwKICAgICAgICAiZHluYW1pY0ZpZWxkVHlwZSI6IG51bGwsCiAgICAgICAgInJldmVyc2VEeW5Tb3J0IjogbnVsbCwKICAgICAgICAicGFyYW1ldGVyVHlwZSI6ICJTSU1QTEUiLAogICAgICAgICJsYWJlbCI6ICJ0ZXN0IiwKICAgICAgICAiZGVmYXVsdFZhbHVlIjogIkxhYmVsIiwKICAgICAgICAidmFsdWVzVG9SZWFkYWJsZVN0cmluZ3MiOiB7CiAgICAgICAgICAiTGFiZWwiOiAidGVzdCIKICAgICAgICB9LAogICAgICAgICJzZWxlY3RlZExhYmVsIjogIkxhYmVsIiwKICAgICAgICAidmFsdWUiOiAidGVzdCIKICAgICAgfQogICAgfSwKICAgICJ0YWdzIjogewogICAgICAiY3VzdG9tZXJUYWdzIjogWwogICAgICAgICJ0ZXJyYWZvcm0iCiAgICAgIF0KICAgIH0KICB9CgoSEgoJd2F2ZWZyb250EgUzLjEuMQ==
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
