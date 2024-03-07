This is an example resource that has no code transformations but valid code blocks. It should render as-is.

## Example Usage

### Basic Example

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
