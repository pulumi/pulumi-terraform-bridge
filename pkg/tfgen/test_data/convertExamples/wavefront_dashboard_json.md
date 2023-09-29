Provides a Wavefront Dashboard JSON resource. This allows dashboards to be created, updated, and deleted.

## Example Usage

```hcl
resource "wavefront_dashboard_json" "test_dashboard_json" {
  dashboard_json = <<-EOF
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
  EOF
}
```

*
*Note:
** If there are dynamic variables in the Wavefront dashboard json, then these variables must be present in a separate file as mentioned in the section below.

## Reading from an External File

Below is the example dashboard with sections and parameters from an external file.

```hcl
resource "wavefront_dashboard_json" "test_dashboard_json" {

  dashboard_json = templatefile("./<dashboard_file_path>.txt",
                                  {
                                    section1   = file("<path>/section1.json"),
                                    section2   = file("<path>/section2.json"),
                                    parameters = file("<path>/params.json")
                                  }
                                )
}
```

The sample files are listed below.

* dashboard_file.txt

```hcl
{
  "name": "Terraform Test Dashboard JSON",
  "description": "a",
  "eventFilterType": "BYCHART",
  "eventQuery": "",
  "defaultTimeWindow": "",
  "url": "tftestimport",
  "displayDescription": false,
  "displaySectionTableOfContents": true,
  "displayQueryParameters": false,
  "sections": [
        ${section1},
        ${section2}
      ],
  "parameterDetails": ${parameters},
  "tags": {
    "customerTags": ["terraform"]
  }
}
```

* section1.json

```json
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
          "summarization": "MEAN"
        }
      ],
      "heightFactor": 50
    }
  ]
}
```

* parameters.json

```json
{
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
}
```

## Import

Dashboard JSON can be imported by using the `id`, e.g.:

```sh<break>
$ pulumi import wavefront:index/dashboardJson:DashboardJson dashboard_json tftestimport
<break>```<break>
