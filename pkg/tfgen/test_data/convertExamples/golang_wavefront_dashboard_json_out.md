Provides a Wavefront Dashboard JSON resource. This allows dashboards to be created, updated, and deleted.

## Example Usage

```go
package main

import (
	"github.com/pulumi/pulumi-wavefront/sdk/go/wavefront"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := wavefront.NewWavefront_dashboard_json(ctx, "testDashboardJson", &wavefront.Wavefront_dashboard_jsonArgs{
			DashboardJson: `{
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
`,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```

*
*Note:
** If there are dynamic variables in the Wavefront dashboard json, then these variables must be present in a separate file as mentioned in the section below.

## Import

Dashboard JSON can be imported by using the `id`, e.g.:

```sh
$ pulumi import wavefront:index/dashboardJson:DashboardJson dashboard_json tftestimport
```
